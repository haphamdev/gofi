package main

import (
	"fmt"
	"io/ioutil"
	"os"
)

func main() {
	currentDir, err := os.Getwd()

	panicIfError(err)

	fmt.Printf("Current working directory is: %s\n", currentDir)

	files, err := ioutil.ReadDir(currentDir)

	panicIfError(err)

	for _, file := range files {
		fileStat, err := os.Stat(fmt.Sprintf("%s/%s", currentDir, file.Name()))
		panicIfError(err)

		fmt.Printf("%s %s\n",
			fileStat.Mode(),
			file.Name(),
		)
	}
}

func panicIfError(err error) {
	if err != nil {
		panic(err)
	}
}
