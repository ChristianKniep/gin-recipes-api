package handlers

import (
	"context"
	"fmt"
	"local/gin/gin-recipes-api/models"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/opengintracing"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type RecipesHandler struct {
	collection *mongo.Collection
	ctx        context.Context
}

func NewRecipesHandler(ctx context.Context, col *mongo.Collection) *RecipesHandler {
	return &RecipesHandler{
		collection: col,
		ctx:        ctx,
	}
}

// swagger:operation GET /recipes recipes listRecipes
// Returns list of recipes from backend
// ---
// produces:
// - aplication/json
// responses:
//    '200':
//         description: Sucessful operation
func (h *RecipesHandler) ListRecipesHandler(c *gin.Context) {
	span := opengintracing.MustGetSpan(c)
	sp := opentracing.StartSpan(
		"ListRecipesHandler",
		opentracing.ChildOf(span.Context()))
	defer sp.Finish()
	sp_find := opentracing.StartSpan(
		"MongoDB.Find",
		opentracing.ChildOf(sp.Context()))
	cur, err := h.collection.Find(h.ctx, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		sp_find.Finish()
		return
	}
	sp_find.Finish()
	defer cur.Close(h.ctx)
	recipes := make([]models.Recipe, 0)
	sp_for := opentracing.StartSpan(
		"LoopMongoDBResult",
		opentracing.ChildOf(sp.Context()))
	for cur.Next(h.ctx) {
		var recipe models.Recipe
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
func (h *RecipesHandler) UpdateRecipeHandler(c *gin.Context) {
	span := opengintracing.MustGetSpan(c)
	sp := opentracing.StartSpan(
		"UpdateRecipeHandler",
		opentracing.ChildOf(span.Context()))
	defer sp.Finish()
	sp_json := opentracing.StartSpan(
		"CreateJSON",
		opentracing.ChildOf(sp.Context()))
	id := c.Param("id")
	var recipe models.Recipe
	if err := c.ShouldBindJSON(&recipe); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		sp_json.Finish()
		return
	}
	rID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("ID is not valid: %s", err.Error())})
		sp_json.Finish()
		return
	}
	recipe.ID = rID
	sp_json.Finish()

	sp_update := opentracing.StartSpan(
		"MongoDB.UpdateOne",
		opentracing.ChildOf(sp.Context()))
	_, err = h.collection.UpdateOne(h.ctx, bson.M{"_id": rID}, bson.D{{"$set", bson.D{
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
func (h *RecipesHandler) NewRecipeHandler(c *gin.Context) {
	span := opengintracing.MustGetSpan(c)
	sp := opentracing.StartSpan(
		"ListRecipesHandler",
		opentracing.ChildOf(span.Context()))
	defer sp.Finish()
	sp_json := opentracing.StartSpan(
		"CreateJSON",
		opentracing.ChildOf(sp.Context()))
	var recipe models.Recipe
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
	_, err := h.collection.InsertOne(h.ctx, recipe)
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
func (h *RecipesHandler) DeleteRecipeHandler(c *gin.Context) {
	span := opengintracing.MustGetSpan(c)
	sp := opentracing.StartSpan(
		"ListRecipesHandler",
		opentracing.ChildOf(span.Context()))
	defer sp.Finish()
	id := c.Param("id")
	rID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		sp_res := opentracing.StartSpan("c.JSON()", opentracing.ChildOf(sp.Context()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("ID is not valid: %s", err.Error())})
		sp_res.Finish()
		return
	}
	sp_del := opentracing.StartSpan("MongoDB.DeleteOne", opentracing.ChildOf(sp.Context()))
	_, err = h.collection.DeleteOne(h.ctx, bson.M{"_id": rID})
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
func (h *RecipesHandler) SearchRecipeHandler(c *gin.Context) {
	span := opengintracing.MustGetSpan(c)
	sp := opentracing.StartSpan(
		"ListRecipesHandler",
		opentracing.ChildOf(span.Context()))
	defer sp.Finish()
	sp_find := opentracing.StartSpan(
		"MongoDB.Find",
		opentracing.ChildOf(sp.Context()))
	tags := strings.Split(c.Query("tag"), ";")
	cur, err := h.collection.Find(h.ctx, bson.M{"tags": bson.M{"$in": tags}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		sp_find.Finish()
		return
	}
	sp_find.Finish()
	defer cur.Close(h.ctx)
	recipes := make([]models.Recipe, 0)
	sp_for := opentracing.StartSpan(
		"LoopMongoDBResult",
		opentracing.ChildOf(sp.Context()))
	for cur.Next(h.ctx) {
		var recipe models.Recipe
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
