package main

import (
    "github.com/getwe/goose"
)


func main() {

    app := goose.NewGoose()
    app.SetIndexStrategy(new(StyIndexer))
    app.SetSearchStrategy(new(StySearcher))
    app.Run()
}
