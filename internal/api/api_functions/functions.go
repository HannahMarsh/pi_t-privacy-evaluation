package api_functions

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/config"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/api/structs"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"golang.org/x/exp/slog"
	"io"
	"net/http"
	"time"
)

type item struct {
	to     string
	from   string
	onion  string
	result chan error
}

// sendOnion sends an onion to the specified address with compression and timeout
func SendOnion(onion structs.Onion) error {
	slog.Info(pl.GetFuncName()+": Sending onion...", "from", config.AddressToName(onion.From), "to", config.AddressToName(onion.To))
	url := fmt.Sprintf("%s/receive", onion.To)

	payload, err := json.Marshal(onion)
	if err != nil {
		return pl.WrapError(err, "%s: failed to marshal onion", pl.GetFuncName())
	}

	compressedBuffer, err := utils.Compress(payload)
	if err != nil {
		return pl.WrapError(err, "%s: failed to compress onion", pl.GetFuncName())
	}

	client := &http.Client{
		Timeout: 30 * time.Second, // Set timeout
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url, &compressedBuffer)
	if err != nil {
		return pl.WrapError(err, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	resp, err := client.Do(req)
	if err != nil {
		return pl.WrapError(err, "%s: failed to send POST request with onion to %s", pl.GetFuncName(), config.AddressToName(onion.To))
	}

	defer func(Body io.ReadCloser) {
		if err = Body.Close(); err != nil {
			slog.Error("Error closing response body", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return pl.NewError("%s: failed to send to first node(url=%s), status code: %d, status: %s", pl.GetFuncName(), url, resp.StatusCode, resp.Status)
	}

	slog.Info("✅ Successfully sent onion. ", "from", config.AddressToName(onion.From), "to", config.AddressToName(onion.To))
	return nil
}

func HandleReceiveOnion(w http.ResponseWriter, r *http.Request, receiveFunction func(structs.Onion) error) {

	var body []byte
	var err error

	// Check if the request is gzipped
	if r.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(r.Body)
		if err != nil {
			slog.Error("Error creating gzip reader", err)
			http.Error(w, "Failed to read gzip content", http.StatusBadRequest)
			return
		}
		defer func(gzipReader *gzip.Reader) {
			if err := gzipReader.Close(); err != nil {
				slog.Error("Error closing gzip reader", err)
			}
		}(gzipReader)

		body, err = io.ReadAll(gzipReader)
		if err != nil {
			slog.Error("Error reading gzip content", err)
			http.Error(w, "Failed to read gzip content", http.StatusBadRequest)
			return
		}
	} else {
		body, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "unable to read body", http.StatusInternalServerError)
			return
		}
	}

	var o structs.Onion
	if err := json.Unmarshal(body, &o); err != nil {
		slog.Error("Error decoding onion", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err = receiveFunction(o); err != nil {
		slog.Error("Error receiving onion", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
