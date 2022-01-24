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

	"github.com/go-redis/redis"

	ginopentracing "github.com/Bose/go-gin-opentracing"
	"github.com/gin-contrib/opengintracing"
	"github.com/gin-contrib/sessions"
	redisStore "github.com/gin-contrib/sessions/redis"
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
	authHandler    *handlers.AuthHandler
)

func init() {
	ctx := context.Background()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://root:example@localhost:27017"))
	if err = client.Ping(context.TODO(), readpref.Primary()); err != nil {
		log.Fatal(err)
	}
	log.Println("Connected to MongoDB")
	mDb := os.Getenv("MONGO_DATABASE")
	if mDb == "" {
		mDb = "demo"
	}
	collection := client.Database(mDb).Collection("recipes")
	collectionUsers := client.Database(mDb).Collection("users")
	// Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	status := redisClient.Ping()
	fmt.Print(status)
	recipesHandler = handlers.NewRecipesHandler(ctx, collection, redisClient)
	authHandler = handlers.NewAuthHandler(ctx, collectionUsers)
	var itemCount int64
	itemCount = 0
	itemCount, err = collection.CountDocuments(ctx, bson.D{})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%d items in collection 'recipes", itemCount)

	if itemCount == -1 {
		recipes := make([]models.Recipe, 0)
		file, err := ioutil.ReadFile("recipes.json")
		if err != nil {
			log.Fatal(err)
		}
		err = json.Unmarshal([]byte(file), &recipes)
		if err != nil {
			log.Fatal(err)
		}
		var listOfRecipes []interface{}
		for _, recipe := range recipes {
			listOfRecipes = append(listOfRecipes, recipe)
		}
		insertManyResult, err := collection.InsertMany(ctx, listOfRecipes)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Insert recipes: ", len(insertManyResult.InsertedIDs))
	}

}

func main() {
	router := gin.Default()
	// RedisStore for user sessions
	store, _ := redisStore.NewStore(10, "tcp", "localhost:6379", "", []byte("secret"))
	router.Use(sessions.Sessions("recipes_api", store))
	p := ginprometheus.NewPrometheus("gin")
	p.Use(router)
	// initialize the global singleton for tracing...
	tracer, reporter, closer, err := ginopentracing.InitTracing("gin", "localhost:5775", ginopentracing.WithEnableInfoLog(true))
	if err != nil {
		panic("unable to init tracing")
	}
	defer closer.Close()
	defer reporter.Close()
	opentracing.SetGlobalTracer(tracer)
	router.GET("/recipes", opengintracing.NewSpan(tracer, "GET:/recipes"), recipesHandler.ListRecipesHandler)
	router.GET("/recipes/search", opengintracing.NewSpan(tracer, "GET:/recipes/search"), recipesHandler.SearchRecipeHandler)
	router.POST("/signin", opengintracing.NewSpan(tracer, "POST:/signin"), authHandler.SignInHandler)
	router.POST("/signout", opengintracing.NewSpan(tracer, "POST:/signout"), authHandler.SignOutHandler)
	router.POST("/refresh", opengintracing.NewSpan(tracer, "POST:/refresh"), authHandler.RefreshHandler)

	authorized := router.Group("/")
	authorized.Use(authHandler.AuthMiddleware())
	// create the middleware
	authorized.POST("/recipes", opengintracing.NewSpan(tracer, "POST:/recipes"), recipesHandler.NewRecipeHandler)
	authorized.PUT("/recipes/:id", opengintracing.NewSpan(tracer, "PUT:/recipes/:id"), recipesHandler.UpdateRecipeHandler)
	authorized.DELETE("/recipes/:id", opengintracing.NewSpan(tracer, "DELETE:/recipes/:id"), recipesHandler.DeleteRecipeHandler)
	router.Run()
}
