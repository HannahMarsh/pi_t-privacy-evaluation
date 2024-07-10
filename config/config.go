package config

import (
	"context"
	"fmt"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/HannahMarsh/PrettyLogger"
	"github.com/ilyakaznacheev/cleanenv"
)

type Adversary struct {
	Gamma          float64 `yaml:"gamma"`
	Chi            float64 `yaml:"chi"`
	Theta          float64 `yaml:"theta"`
	NodeIDs        []int   `yaml:"nodeIDs"`
	AlwaysDropFrom []int   `yaml:"alwaysDropFrom"`
}

type BulletinBoard struct {
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Address string
}

type Node struct {
	ID      int    `yaml:"id"`
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Address string
}

type Client struct {
	ID      int    `yaml:"id"`
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Address string
}

type Metrics struct {
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Address string
}

type Scenario struct {
	Name     string    `yaml:"name"`
	Messages []Message `yaml:"messages"`
}

type Message struct {
	From    int    `yaml:"from"`
	To      int    `yaml:"to"`
	Content string `yaml:"content"`
}

type Config struct {
	Epsilon       float64       `yaml:"epsilon"`
	Delta         float64       `yaml:"delta"`
	L             int           `yaml:"L"`
	N             int           `yaml:"N"`
	R             int           `yaml:"R"`
	D             int           `yaml:"D"`
	StdDev        float64       `yaml:"stdDev"`
	BulletinBoard BulletinBoard `yaml:"bulletin_board"`
	Nodes         []Node        `yaml:"nodes"`
	Metrics       Metrics       `yaml:"metrics"`
	Clients       []Client      `yaml:"clients"`
	Adversary     Adversary     `yaml:"adversary"`
	Scenarios     []Scenario    `yaml:"scenarios"`
}

func (cnfg *Config) GetClientAddress(id int) string {
	if f := utils.Find(cnfg.Clients, func(client Client) bool {
		return client.ID == id
	}); f != nil {
		return f.Address
	} else {
		PrettyLogger.LogNewError("Client not found with id=%d", id)
		return ""
	}
}

var GlobalConfig *Config
var GlobalCtx context.Context
var GlobalCancel context.CancelFunc
var Names sync.Map

func InitGlobal() (err error) {
	GlobalCtx, GlobalCancel = context.WithCancel(context.Background())
	GlobalConfig = &Config{}
	scenarios := &Config{}

	if err, scenarios = readConfig(scenarios, "scenarios.yml"); err != nil {
		return PrettyLogger.WrapError(err, "config.NewConfig(): global config error")
	}
	if err, GlobalConfig = readConfig(GlobalConfig, "config.yml"); err != nil {
		return PrettyLogger.WrapError(err, "config.NewConfig(): global config error")
	}
	if err = cleanenv.ReadEnv(GlobalConfig); err != nil {
		return PrettyLogger.WrapError(err, "config.NewConfig(): global config error")
	}

	// Update node addresses
	for i := range GlobalConfig.Nodes {
		GlobalConfig.Nodes[i].Address = fmt.Sprintf("http://%s:%d", GlobalConfig.Nodes[i].Host, GlobalConfig.Nodes[i].Port)
	}

	// Update client addresses
	for i := range GlobalConfig.Clients {
		GlobalConfig.Clients[i].Address = fmt.Sprintf("http://%s:%d", GlobalConfig.Clients[i].Host, GlobalConfig.Clients[i].Port)
	}
	GlobalConfig.BulletinBoard.Address = fmt.Sprintf("http://%s:%d", GlobalConfig.BulletinBoard.Host, GlobalConfig.BulletinBoard.Port)
	GlobalConfig.Metrics.Address = fmt.Sprintf("http://%s:%d", GlobalConfig.Metrics.Host, GlobalConfig.Metrics.Port)
	GlobalConfig.Scenarios = scenarios.Scenarios

	GlobalConfig.Nodes = GlobalConfig.Nodes[:GlobalConfig.N]
	GlobalConfig.Clients = GlobalConfig.Clients[:GlobalConfig.R]
	return nil
}

func readConfig(cfg *Config, file string) (error, *Config) {
	// path = "/config/config.yml"
	var dir string
	var err error
	if dir, err = os.Getwd(); err != nil {
		return PrettyLogger.WrapError(err, "config.NewConfig(): global config error"), nil
	} else if err = cleanenv.ReadConfig(dir+"/config/"+file, cfg); err != nil {
		if _, currentFile, _, ok := runtime.Caller(0); !ok { // Get the absolute path of the current file
			return PrettyLogger.NewError("Failed to get current file path"), nil
		} else if err = cleanenv.ReadConfig(filepath.Join(filepath.Dir(currentFile), "/"+file), cfg); err != nil {
			return PrettyLogger.WrapError(err, "config.NewConfig(): global config error"), nil
		}
	}
	return nil, cfg
}

func HostPortToName(host string, port int) string {
	return AddressToName(fmt.Sprintf("http://%s:%d", host, port))
}

var PurpleColor = "\033[35m"
var OrangeColor = "\033[33m"
var ResetColor = "\033[0m"

func AddressToName(address string) string {
	if name, ok := Names.Load(address); ok {
		return name.(string)
	}
	if strings.Count(address, "/") > 2 {
		spl := strings.Split(address, "/")
		address = spl[0] + "//" + spl[1]
	}
	if name, ok := Names.Load(address); ok {
		return name.(string)
	}
	for _, node := range GlobalConfig.Nodes {
		if address == node.Address {
			name := fmt.Sprintf("%sNode %d%s", PurpleColor, node.ID, ResetColor)
			Names.Store(address, name)
			return name
		}
	}
	for _, client := range GlobalConfig.Clients {
		if address == client.Address {
			name := fmt.Sprintf("%sClient %d%s", OrangeColor, client.ID, ResetColor)
			Names.Store(address, name)
			return name
		}
	}
	if address == GlobalConfig.BulletinBoard.Address {
		name := "Bulletin Board"
		Names.Store(address, name)
		return name
	}
	if address == GlobalConfig.Metrics.Address {
		name := "Metrics"
		Names.Store(address, name)
		return name
	}
	return address
}
