package structs

import (
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/config"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"strings"
)

type PublicNodeApi struct {
	ID      int
	Address string
}

func GetPublicNodeApi(address string) PublicNodeApi {
	if nodeCfg := utils.Find(config.GlobalConfig.Nodes, func(node config.Node) bool {
		return strings.Contains(node.Address, address) || strings.Contains(address, node.Address)
	}); nodeCfg != nil {
		return PublicNodeApi{
			ID:      nodeCfg.ID,
			Address: nodeCfg.Address,
		}
	}
	if clientCfg := utils.Find(config.GlobalConfig.Clients, func(client config.Client) bool {
		return strings.Contains(client.Address, address) || strings.Contains(address, client.Address)
	}); clientCfg != nil {
		return PublicNodeApi{
			ID:      clientCfg.ID,
			Address: clientCfg.Address,
		}
	}
	pl.LogNewError("Could not find node with address: %s", address)
	return PublicNodeApi{
		Address: address,
	}
}

func GetPublicNodeApiFromID(id int) PublicNodeApi {
	if nodeCfg := utils.Find(config.GlobalConfig.Nodes, func(node config.Node) bool {
		return node.ID == id
	}); nodeCfg != nil {
		return PublicNodeApi{
			ID:      nodeCfg.ID,
			Address: nodeCfg.Address,
		}
	}
	return PublicNodeApi{
		ID: id,
	}
}
