package mux

type maddr string

func (a maddr) Network() string {
	return string(a)
}

func (a maddr) String() string {
	return string(a)
}
