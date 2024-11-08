package config

import (
	"flag"
)

type AppConfig struct {
	Port      int
	NodeTCP   int
	NodeUDP   int
	Bootstrap string
}

func Parse() AppConfig {
	conf := AppConfig{}
	flag.IntVar(&conf.Port, "p", 8080, "The port of the http interface.")
	flag.IntVar(&conf.NodeTCP, "p-tcp", 8079, "The port of the distributed node TCP interface.")
	flag.IntVar(&conf.NodeUDP, "p-udp", 8078, "The port of the distributed node UDP interface.")
	flag.StringVar(&conf.Bootstrap, "b", "", "The Multiaddress of the bootstrap node")
	flag.Parse()
	return conf
}
