package structs

type StartRunAPI struct {
	Nodes    []PublicNodeApi
	Clients  []PublicNodeApi
	Scenario int
}
