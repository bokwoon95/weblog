package main

import (
	"fmt"
	"log"
	"os"

	"github.com/bokwoon95/weblog/pagemanager/renderly"
	"github.com/pelletier/go-toml"
)

type Page struct {
	HTML   string            `toml:"html"`
	CSS    []string          `toml:"css"`
	JS     []string          `toml:"js"`
	Target string            `toml:"target"`
	Data   map[string]string `toml:"data"`
}

type Config struct {
	Author string `toml:"author"`
	Pages  []Page `toml:"pages"`
}

func main() {
	dir := renderly.AbsDir(".")
	data, err := os.ReadFile(dir + "config.toml")
	if err != nil {
		log.Fatalln(err)
	}
	var config Config
	err = toml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Printf("%+v\n", config)
}
