package structs

import (
	"github.com/HannahMarsh/pi_t-privacy-evaluation/config"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
)

type PublicNodeApi struct {
	ID      int
	Address string
}

func GetPublicNodeApi(address string) PublicNodeApi {
	if nodeCfg := utils.Find(config.GlobalConfig.Nodes, func(node config.Node) bool {
		return node.Address == address
	}); nodeCfg != nil {
		return PublicNodeApi{
			ID:      nodeCfg.ID,
			Address: address,
		}
	}
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
