package main

import "flag"

var configFile = flag.String("config", "$HOME/.0x35-tool.json", "the path to the config file")

func main() {
	flag.Parse()
}
