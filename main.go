package main

import "github.com/jamescrawford/jamfschool2snipeIT/cmd"

var version = "dev"

func main() {
	cmd.Version = version
	cmd.Execute()
}
