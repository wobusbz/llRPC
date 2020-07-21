package regirsty

type Service struct {
	Name  string
	Nodes []*Node
}

type Node struct {
	Id     string `json:"id"`
	Ip     string `json:"ip"`
	Port   int    `json:"port"`
	WeigHt int    `json:"weigHt"`
}
