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
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	ginopentracing "github.com/Bose/go-gin-opentracing"
	"github.com/gin-contrib/opengintracing"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	ginprometheus "github.com/zsais/go-gin-prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var recipes []Recipe

var (
	ctx    context.Context
	err    error
	client *mongo.Client
)

func init() {
	recipes = make([]Recipe, 0)
	file, _ := ioutil.ReadFile("recipes.json")
	_ = json.Unmarshal([]byte(file), &recipes)
	ctx = context.Background()
	client, err = mongo.Connect(ctx, options.Client().ApplyURI("mongodb://root:example@localhost:27017"))
	if err = client.Ping(context.TODO(), readpref.Primary()); err != nil {
		log.Fatal(err)
	}
	log.Println("Connected to MongoDB")
	var listOfRecipes []interface{}
	for _, recipe := range recipes {
		listOfRecipes = append(listOfRecipes, recipe)
	}
	collection := client.Database(os.Getenv("MONDO_DATABASE")).Collection("recipes")
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
	ot := ginopentracing.OpenTracer([]byte("api-request-"))
	router.Use(ot)
	router.POST("/recipes", opengintracing.NewSpan(tracer, "POST:/recipes"), NewRecipeHandler)
	router.GET("/recipes", opengintracing.NewSpan(tracer, "GET:/recipes"), ListRecipesHandler)
	router.PUT("/recipes/:id", opengintracing.NewSpan(tracer, "PUT:/recipes/:id"), UpdateRecipeHandler)
	router.DELETE("/recipes/:id", opengintracing.NewSpan(tracer, "DELETE:/recipes/:id"), DeleteRecipeHandler)
	router.GET("/recipes/search", opengintracing.NewSpan(tracer, "GET:/recipes/search"), SearchRecipeHandler)
	router.Run()
}

// swagger:parameters recipes newRecipe
type Recipe struct {
	//swagger:ignore
	ID           primitive.ObjectID `json:"id" bson:"_id"`
	Name         string             `json:"name" bson:"name"`
	Tags         []string           `json:"tags" bson:"tags"`
	Ingredients  []string           `json:"ingredients" bson:"ingredients"`
	Instructions []string           `json:"instructions" bson:"instructions"`
	PublishedAt  time.Time          `json:"publishedAt" bson:"publishedAt"`
}

// swagger:operation PUT /recipes/{id} recipes updateRecipes
// Updates an existing recipe
// ---
// parameters:
// - name: id
//   in: path
//   description: ID of recipe
//   required: true
//   type: string
// produces:
// - aplication/json
// responses:
//   '200':
//         description: Sucessful operation
//   '400':
//         description: invalid input
//   '404':
//         description: Invalid recipe ID
func UpdateRecipeHandler(c *gin.Context) {
	span := opengintracing.MustGetSpan(c)
	sp := opentracing.StartSpan(
		"UpdateRecipeHandler",
		opentracing.ChildOf(span.Context()))
	defer sp.Finish()
	sp_con := opentracing.StartSpan(
		"MongoDB.Connect",
		opentracing.ChildOf(sp.Context()))
	collection := client.Database(os.Getenv("MONDO_DATABASE")).Collection("recipes")
	sp_con.Finish()
	sp_json := opentracing.StartSpan(
		"CreateJSON",
		opentracing.ChildOf(sp.Context()))
	id := c.Param("id")
	var recipe Recipe
	if err := c.ShouldBindJSON(&recipe); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		sp_json.Finish()
		return
	}
	rID, e := primitive.ObjectIDFromHex(id)
	if e != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("ID is not valid: %s", err.Error())})
		sp_json.Finish()
		return
	}
	recipe.ID = rID
	sp_json.Finish()

	sp_update := opentracing.StartSpan(
		"MongoDB.UpdateOne",
		opentracing.ChildOf(sp.Context()))
	_, err = collection.UpdateOne(ctx, bson.M{"_id": rID}, bson.D{{"$set", bson.D{
		{"name", recipe.Name},
		{"tags", recipe.Tags},
		{"ingredients", recipe.Ingredients},
		{"instructions", recipe.Instructions},
	}}})
	sp_update.Finish()
	if err != nil {
		log.Println(err.Error())
		sp_res := opentracing.StartSpan(
			"c.JSON()",
			opentracing.ChildOf(sp.Context()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		sp_res.Finish()
		return
	}
	sp_res := opentracing.StartSpan(
		"c.JSON()",
		opentracing.ChildOf(sp.Context()))
	c.JSON(http.StatusOK, gin.H{"message": "Recipe has been updated"})
	sp_res.Finish()
}

// swagger:operation POST /recipes recipes newRecipe
// Create a new recipe
// ---
// produces:
// - application/json
// responses:
//     '200':
//         description: Successful operation
//     '400':
//         description: Invalid input
func NewRecipeHandler(c *gin.Context) {
	span := opengintracing.MustGetSpan(c)
	sp := opentracing.StartSpan(
		"ListRecipesHandler",
		opentracing.ChildOf(span.Context()))
	defer sp.Finish()
	sp_con := opentracing.StartSpan(
		"MongoDB.Connect",
		opentracing.ChildOf(sp.Context()))
	collection := client.Database(os.Getenv("MONDO_DATABASE")).Collection("recipes")
	sp_con.Finish()
	sp_json := opentracing.StartSpan(
		"CreateJSON",
		opentracing.ChildOf(sp.Context()))
	var recipe Recipe
	if err := c.ShouldBindJSON(&recipe); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		sp_json.Finish()
		return
	}
	sp_json.Finish()
	sp_ins := opentracing.StartSpan(
		"MongoDB.InsertOne",
		opentracing.ChildOf(sp.Context()))
	recipe.ID = primitive.NewObjectID()
	recipe.PublishedAt = time.Now()
	_, err = collection.InsertOne(ctx, recipe)
	if err != nil {
		log.Println(err.Error())
		sp_res := opentracing.StartSpan(
			"c.JSON()",
			opentracing.ChildOf(sp.Context()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Error while inserting new recipe: %s", err.Error()),
		})
		sp_res.Finish()
		return
	}
	sp_ins.Finish()
	sp_res := opentracing.StartSpan(
		"c.JSON()",
		opentracing.ChildOf(sp.Context()))
	c.JSON(http.StatusOK, recipe)
	sp_res.Finish()
}

// swagger:operation GET /recipes recipes listRecipes
// Returns list of recipes
// ---
// produces:
// - aplication/json
// responses:
//    '200':
//         description: Sucessful operation
func ListRecipesHandler(c *gin.Context) {
	span := opengintracing.MustGetSpan(c)
	sp := opentracing.StartSpan(
		"ListRecipesHandler",
		opentracing.ChildOf(span.Context()))
	defer sp.Finish()
	sp_find := opentracing.StartSpan(
		"MongoDB.Find",
		opentracing.ChildOf(sp.Context()))
	collection := client.Database(os.Getenv("MONDO_DATABASE")).Collection("recipes")
	cur, err := collection.Find(ctx, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		sp_find.Finish()
		return
	}
	sp_find.Finish()
	defer cur.Close(ctx)
	recipes := make([]Recipe, 0)
	sp_for := opentracing.StartSpan(
		"LoopMongoDBResult",
		opentracing.ChildOf(sp.Context()))
	for cur.Next(ctx) {
		var recipe Recipe
		err = cur.Decode(&recipe)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			sp_for.Finish()
			return
		}
		recipes = append(recipes, recipe)
	}
	sp_for.Finish()
	sp_res := opentracing.StartSpan(
		"c.JSON()",
		opentracing.ChildOf(sp.Context()))
	c.JSON(http.StatusOK, recipes)
	sp_res.Finish()
}

// swagger:operation DELETE /recipes/{id} recipes deleteRecipe
// Delete an existing recipe
// ---
// produces:
// - application/json
// parameters:
//   - name: id
//     in: path
//     description: ID of the recipe
//     required: true
//     type: string
// responses:
//     '200':
//         description: Successful operation
//     '404':
//         description: Invalid recipe ID
func DeleteRecipeHandler(c *gin.Context) {
	span := opengintracing.MustGetSpan(c)
	sp := opentracing.StartSpan(
		"ListRecipesHandler",
		opentracing.ChildOf(span.Context()))
	defer sp.Finish()
	sp_con := opentracing.StartSpan(
		"MongoDB.Connect",
		opentracing.ChildOf(sp.Context()))
	collection := client.Database(os.Getenv("MONDO_DATABASE")).Collection("recipes")
	sp_con.Finish()
	id := c.Param("id")
	rID, e := primitive.ObjectIDFromHex(id)
	if e != nil {
		sp_res := opentracing.StartSpan("c.JSON()", opentracing.ChildOf(sp.Context()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("ID is not valid: %s", err.Error())})
		sp_res.Finish()
		return
	}
	sp_del := opentracing.StartSpan("MongoDB.DeleteOne", opentracing.ChildOf(sp.Context()))
	_, err := collection.DeleteOne(ctx, bson.M{"_id": rID})
	sp_del.Finish()
	if err != nil {
		sp_res := opentracing.StartSpan("c.JSON()", opentracing.ChildOf(sp.Context()))
		c.JSON(http.StatusNotFound, gin.H{"error": "Recipe not found"})
		sp_res.Finish()
		return
	}
	sp_res := opentracing.StartSpan("c.JSON()", opentracing.ChildOf(sp.Context()))
	c.JSON(http.StatusOK, gin.H{"message": "Recipe has been deleted"})
	sp_res.Finish()
}

// swagger:operation GET /recipes/search recipes findRecipe
// Search recipes based on tags
// ---
// produces:
// - application/json
// parameters:
//   - name: tag
//     in: query
//     description: recipe tag
//     required: true
//     type: string
// responses:
//     '200':
//         description: Successful operation
func SearchRecipeHandler(c *gin.Context) {
	span := opengintracing.MustGetSpan(c)
	sp := opentracing.StartSpan(
		"ListRecipesHandler",
		opentracing.ChildOf(span.Context()))
	defer sp.Finish()
	sp_con := opentracing.StartSpan(
		"MongoDB.Connect",
		opentracing.ChildOf(sp.Context()))
	collection := client.Database(os.Getenv("MONDO_DATABASE")).Collection("recipes")
	sp_con.Finish()
	sp_find := opentracing.StartSpan(
		"MongoDB.Find",
		opentracing.ChildOf(sp.Context()))
	tags := strings.Split(c.Query("tag"), ";")
	cur, err := collection.Find(ctx, bson.M{"tags": bson.M{"$in": tags}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		sp_find.Finish()
		return
	}
	sp_find.Finish()
	defer cur.Close(ctx)
	recipes := make([]Recipe, 0)
	sp_for := opentracing.StartSpan(
		"LoopMongoDBResult",
		opentracing.ChildOf(sp.Context()))
	for cur.Next(ctx) {
		var recipe Recipe
		err = cur.Decode(&recipe)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			sp_for.Finish()
			return
		}
		recipes = append(recipes, recipe)
	}
	sp_for.Finish()
	sp_res := opentracing.StartSpan(
		"c.JSON()",
		opentracing.ChildOf(sp.Context()))
	c.JSON(http.StatusOK, recipes)
	sp_res.Finish()
}
