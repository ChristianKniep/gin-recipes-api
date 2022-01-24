package handlers

import (
	"context"
	"crypto/sha256"
	"local/gin/gin-recipes-api/models"
	"net/http"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-contrib/opengintracing"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/xid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type AuthHandler struct {
	collection *mongo.Collection
	ctx        context.Context
}

func NewAuthHandler(ctx context.Context, collection *mongo.Collection) *AuthHandler {
	return &AuthHandler{
		collection: collection,
		ctx:        ctx,
	}
}

type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

type JWTOutput struct {
	Token   string    `json:"token"`
	Expires time.Time `json:"expires"`
}

func (h *AuthHandler) SignOutHandler(c *gin.Context) {
	span := opengintracing.MustGetSpan(c)
	sp := NewSubSpan(span, "SignOutHandler")
	sp_session := NewSubSpan(sp, "ClearSession")
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	sp_session.Finish()
	sp_res := NewSubSpan(sp, "c.JSON()")
	c.JSON(http.StatusOK, gin.H{"message": "User logged out"})
	sp_res.Finish()
}

func (h *AuthHandler) SignInHandler(c *gin.Context) {
	span := opengintracing.MustGetSpan(c)
	sp := NewSubSpan(span, "UpdateRecipeHandler")
	defer sp.Finish()
	var user models.User
	sp_json := NewSubSpan(sp, "BindJSON(user)")
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		sp_json.Finish()
	}
	sp_json.Finish()
	sp_auth := NewSubSpan(sp, "AuthUser")
	hash := sha256.New()
	sp_mdb := NewSubSpan(sp_auth, "MongoDB.FindUser()")
	cur := h.collection.FindOne(h.ctx, bson.M{
		"username": user.Username,
		"password": string(hash.Sum([]byte(user.Password))),
	})
	sp_mdb.Finish()
	if cur.Err() != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		sp_auth.Finish()
		return
	}
	sp_auth.Finish()
	/*sp_token := NewSubSpan(sp, "CreateToken")
	expirationTime := time.Now().Add(10 * time.Minute)
	claims := &Claims{
		Username: user.Username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = ""
	}
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		sp_token.Finish()
	}
	jwtOutput := JWTOutput{
		Token:   tokenString,
		Expires: expirationTime,
	}
	sp_token.Finish()
	*/
	sp_session := NewSubSpan(sp, "Session")
	sessionToken := xid.New().String()
	session := sessions.Default(c)
	session.Set("username", user.Username)
	session.Set("token", sessionToken)
	session.Save()
	sp_session.Finish()
	sp_res := NewSubSpan(sp, "c.JSON()")
	c.JSON(http.StatusOK, gin.H{"message": "User signed in"})
	sp_res.Finish()
}

func (h *AuthHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		//tokenValue := c.GetHeader("Authorization")
		session := sessions.Default(c)
		sessionToken := session.Get("token")
		/*claims := &Claims{}
		tkn, err := jwt.ParseWithClaims(tokenValue, claims,
			func(token *jwt.Token) (interface{}, error) {
				return []byte(os.Getenv("JWT_SECRET")), nil
			})
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
		}
		if tkn == nil || !tkn.Valid {
			c.AbortWithStatus(http.StatusUnauthorized)
		}*/
		if sessionToken == nil {
			c.JSON(http.StatusForbidden, gin.H{"message": "Not logged in"})
			c.Abort()
		}
		c.Next()
	}
}

func (h *AuthHandler) RefreshHandler(c *gin.Context) {
	span := opengintracing.MustGetSpan(c)
	sp := opentracing.StartSpan(
		"RefreshHandler",
		opentracing.ChildOf(span.Context()))
	defer sp.Finish()
	sp_token := NewSubSpan(sp, "CreateToken")
	tokenValue := c.GetHeader("Authorization")
	claims := &Claims{}
	tkn, err := jwt.ParseWithClaims(tokenValue, claims,
		func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		sp_token.Finish()
		return
	}
	if tkn == nil || !tkn.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		sp_token.Finish()
		return
	}
	if time.Unix(claims.ExpiresAt, 0).Sub(time.Now()) > 30*time.Second {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token is not expired yet"})
		sp_token.Finish()
		return
	}
	expirationTime := time.Now().Add(5 * time.Minute)
	claims.ExpiresAt = expirationTime.Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = ""
	}
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		sp_token.Finish()
		return
	}
	jwtOutput := JWTOutput{
		Token:   tokenString,
		Expires: expirationTime,
	}
	sp_token.Finish()
	sp_res := NewSubSpan(sp, "c.JSON()")
	c.JSON(http.StatusOK, jwtOutput)
	sp_res.Finish()
}
