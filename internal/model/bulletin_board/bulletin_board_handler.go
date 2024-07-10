package bulletin_board

import (
	"encoding/json"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/api/structs"
	"golang.org/x/exp/slog"
	"net/http"
)

func (bb *BulletinBoard) HandleRegisterNode(w http.ResponseWriter, r *http.Request) {
	//	slog.Info("Received node registration request")
	var node structs.PublicNodeApi
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		slog.Error("Error decoding node registration request", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	slog.Info("Registering node with", "id", node.ID)
	bb.RegisterNode(node)
	w.WriteHeader(http.StatusCreated)
}

func (bb *BulletinBoard) HandleRegisterClient(w http.ResponseWriter, r *http.Request) {
	slog.Info("Received client registration request")
	var client structs.PublicNodeApi
	if err := json.NewDecoder(r.Body).Decode(&client); err != nil {
		slog.Error("Error decoding client registration request", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	slog.Info("Registering client with", "id", client.ID)
	bb.RegisterClient(client)
	w.WriteHeader(http.StatusCreated)
}
