package streamer

type Port struct {
	c chan []byte
}

type Component struct {
	inputPort  Port
	outputPort Port
	errorPort  Port
}
