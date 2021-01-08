package gen

var runtimeIdToState []Block

type Block struct {
	Name   string
	States map[string]interface{}
}
