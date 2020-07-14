package regirsty

type Service struct {
	Name  string
	Nodes []*Node
}

type Node struct {
	Id   string
	Ip   string
	Port int
}
