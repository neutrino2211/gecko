package config

type CompileCfg struct {
	Arch     string
	Platform string
	CFlags   []string
	CLFlags  []string
	CObjects []string
}
