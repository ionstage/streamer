package streamer

type Port struct {
	c chan []byte
}

func NewPort() *Port {
	return &Port{c: make(chan []byte)}
}

type Component struct {
	inputPorts  []Port
	outputPorts []Port
	errorPorts  []Port
}
