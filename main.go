package main

import (
	"github.com/gin-gonic/gin"
	"github.com/zsais/go-gin-prometheus"
	ginopentracing "github.com/Bose/go-gin-opentracing"
	"github.com/opentracing/opentracing-go"
	"os"
	"fmt"
)

func main() {
   	router := gin.Default()
	p := ginprometheus.NewPrometheus("gin")
	p.Use(router)
	hostName, err := os.Hostname()
	if err != nil {
		hostName = "unknown"
	}
	// initialize the global singleton for tracing...
	tracer, reporter, closer, err := ginopentracing.InitTracing(fmt.Sprintf("go-gin-opentracing-example::%s", hostName), "localhost:5775", ginopentracing.WithEnableInfoLog(true))
	if err != nil {
		panic("unable to init tracing")
	}
	defer closer.Close()
	defer reporter.Close()
	opentracing.SetGlobalTracer(tracer)

	// create the middleware
	ot := ginopentracing.OpenTracer([]byte("api-request-"))
	router.Use(ot)
	router.Run()

}