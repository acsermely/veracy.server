package config

import (
	"flag"
)

type AppConfig struct {
	Port      int
	Bootstrap string
}

func Parse() AppConfig {
	conf := AppConfig{}
	flag.IntVar(&conf.Port, "p", 8080, "The port of the http interface.")
	flag.StringVar(&conf.Bootstrap, "b", "", "The Multiaddress of the bootstrap node")
	flag.Parse()
	return conf
}
