//  Recipes API
//
//  This is a sample recipes API. You can find out more on github.
//
//  Schemes: http
//  Host: localhost:8080
//  BasePath: /
//  Version: 1.0.0
//  Contact: Christian Kniep <go@qnib.org>
//
//  Consumes:
//  - application/json
//
//  Produces:
//  - application/json
// swagger:meta
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"local/gin/gin-recipes-api/handlers"
	"local/gin/gin-recipes-api/models"
	"log"
	"os"

	ginopentracing "github.com/Bose/go-gin-opentracing"
	"github.com/gin-contrib/opengintracing"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	ginprometheus "github.com/zsais/go-gin-prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var (
	recipesHandler *handlers.RecipesHandler
)

func init() {
	ctx := context.Background()

	recipes := make([]models.Recipe, 0)
	file, _ := ioutil.ReadFile("recipes.json")
	_ = json.Unmarshal([]byte(file), &recipes)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://root:example@localhost:27017"))
	if err = client.Ping(context.TODO(), readpref.Primary()); err != nil {
		log.Fatal(err)
	}
	log.Println("Connected to MongoDB")
	var listOfRecipes []interface{}
	for _, recipe := range recipes {
		listOfRecipes = append(listOfRecipes, recipe)
	}
	collection := client.Database(os.Getenv("MONDO_DATABASE")).Collection("recipes")
	recipesHandler = handlers.NewRecipesHandler(ctx, collection)
	var itemCount int64
	itemCount = 0
	itemCount, err = collection.CountDocuments(ctx, bson.D{})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%d items in collection 'recipes", itemCount)

	if itemCount == 0 {
		insertManyResult, err := collection.InsertMany(ctx, listOfRecipes)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Insert recipes: ", len(insertManyResult.InsertedIDs))
	}
}

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
	router.POST("/recipes", opengintracing.NewSpan(tracer, "POST:/recipes"), recipesHandler.NewRecipeHandler)
	router.GET("/recipes", opengintracing.NewSpan(tracer, "GET:/recipes"), recipesHandler.ListRecipesHandler)
	router.PUT("/recipes/:id", opengintracing.NewSpan(tracer, "PUT:/recipes/:id"), recipesHandler.UpdateRecipeHandler)
	router.DELETE("/recipes/:id", opengintracing.NewSpan(tracer, "DELETE:/recipes/:id"), recipesHandler.DeleteRecipeHandler)
	router.GET("/recipes/search", opengintracing.NewSpan(tracer, "GET:/recipes/search"), recipesHandler.SearchRecipeHandler)
	router.Run()
}
