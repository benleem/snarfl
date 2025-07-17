package main

import (
	"fmt"
)

type Config struct {
	port string
}

func NewConfig(port int) *Config {
	fullPort := fmt.Sprintf(":%v", port)
	return &Config{port: fullPort}
}

func main() {
	config := NewConfig(8080)
	fmt.Println(config.port)
}
