package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/thinktt/yowapi/pkg/auth"
	"github.com/thinktt/yowapi/pkg/db"
	"github.com/thinktt/yowapi/pkg/events"
	"github.com/thinktt/yowapi/pkg/games"
	"github.com/thinktt/yowapi/pkg/kingcheck"
	"github.com/thinktt/yowapi/pkg/models"
	"github.com/thinktt/yowapi/pkg/moveque"
	"github.com/thinktt/yowapi/pkg/utils"
)

var cmpMap = make(map[string]models.Cmp)

func main() {

	loadCmps()
	// fmt.Println(cmpMap["Ash"])

	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"}
	config.AllowHeaders = []string{"*"}
	port := os.Getenv("PORT")
	if port == "" {
		fmt.Println("No PORT environment variable detected, defaulting to 8080")
		port = "8080"
	}

	// liveGamesIDs, err := db.GetAllLiveGameIDs()
	// if err != nil {
	// 	fmt.Errorf("Not able to get live games: %s", err.Error())
	// }

	// // make moves for any games that ar waiting for the engine
	// for _, id := range liveGamesIDs {
	// 	go games.PublishGameUpdates(id)
	// }

	r := gin.New()
	r.Use(cors.New(config))

	// catch and remove trailing slashes on routes and start router again
	// this let's us handle trailing slashes without redirecting
	r.RedirectTrailingSlash = false
	r.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		if len(path) > 1 && path[len(path)-1] == '/' {
			c.Request.URL.Path = path[:len(path)-1]
			r.HandleContext(c)
			c.Abort()
			return
		}
		c.Next()
	})

	healthWasCalled := false
	r.Use(func(c *gin.Context) {
		if c.Request.URL.Path == "/health" && healthWasCalled {
			c.Next()
			return
		}
		gin.Logger()(c)
		c.Next()
	})

	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		healthWasCalled = true
		c.JSON(http.StatusOK, gin.H{
			"message": "API is healthy",
		})
	})

	r.POST("/users", func(c *gin.Context) {
		var userReq models.UserRequest
		if err := c.ShouldBindJSON(&userReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if !userIsValid(userReq.ID) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid user ID, does not match user regex"})
			return
		}

		// checking settings for user count limit and king requirement
		settings, err := db.GetSettings()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "DB Error getting settings: " + err.Error()})
			return
		}

		users, err := db.GetAllUsers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "DB Error getting user count: " + err.Error()})
			return
		}

		if users.Count >= settings.UserLimit {
			c.JSON(http.StatusForbidden, gin.H{
				"message": "user limit reached, no new users can be added at this time",
			})
			return
		}

		kingIsRequired := settings.KingIsRequired

		var kingCmVersion string
		if kingIsRequired {
			kingCmVersion, err = kingcheck.GetVersion(userReq.KingBlob)
			if err != nil || kingCmVersion == "" {
				c.JSON(http.StatusBadRequest, gin.H{"message": "king blob is not valid"})
				return
			}
		} else {
			// king is not required, we will set this user as Beta user
			kingCmVersion = "B"
		}

		// user IDs will always be lower case in db
		id := strings.ToLower(userReq.ID)

		user := models.User{
			ID:                    id,
			KingCmVersion:         kingCmVersion,
			HasAcceptedDisclaimer: userReq.HasAcceptedDisclaimer,
		}

		result, err := db.CreateUser(user)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "DB Error: " + err.Error()})
			return
		}

		if result.MatchedCount > 0 {
			c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("user %s already exist, no new creation", user.ID)})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": fmt.Sprintf("user %s successfully added", user.ID)})
	})

	r.GET("/users/:id", func(c *gin.Context) {
		userID := c.Param("id")

		user, err := db.GetUser(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB Error: " + err.Error()})
			return
		}

		if (user == models.User{}) {
			c.JSON(http.StatusNotFound, gin.H{"message": fmt.Sprintf("no user found for id %s", userID)})
			return
		}

		c.JSON(http.StatusOK, user)
	})

	r.POST("/games", func(c *gin.Context) {
		var game models.Game

		// Binding and validation
		if err := c.ShouldBindJSON(&game); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if !userIsValid(game.User) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid user ID, does not match user regex"})
			return
		}

		result, err := db.CreateGame(game)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "DB Error: " + err.Error()})
			return
		}

		// MongoDB already had this ID in the DB, so it didn't create a new one
		if result.MatchedCount > 0 {
			c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("game %s already exist, no new creation", game.ID)})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": fmt.Sprintf("game %s successfully added", game.ID)})
	})

	r.GET("/games/:id", func(c *gin.Context) {
		id := c.Param("id")

		game, err := db.GetGame(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB Error: " + err.Error()})
			return
		}

		if game == nil {
			c.JSON(http.StatusNotFound, gin.H{"message": fmt.Sprintf("no game found for id %s", id)})
			return
		}

		c.JSON(http.StatusOK, game)
	})

	r.GET("/games2/live", func(c *gin.Context) {
		gameIDs, err := db.GetAllLiveGameIDs()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if gameIDs == nil {
			gameIDs = []string{}
		}

		c.JSON(http.StatusOK, gameIDs)
	})

	r.GET("/ids/:user", func(c *gin.Context) {
		user := c.Param("user")

		createdAtParam := c.Query("createdAt")
		createdAt, err := strconv.ParseInt(createdAtParam, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid createdAt parameter"})
			return
		}

		ids, err := db.GetGameIDs(user, createdAt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, ids)
	})

	r.GET("/games2/:id/pgn", func(c *gin.Context) {
		id := c.Param("id")

		pgn, err := games.GetPGN(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error building pgn: " + err.Error()})
			return
		}

		c.String(http.StatusOK, pgn)
	})

	r.GET("/games2/:id", func(c *gin.Context) {
		id := c.Param("id")

		game, err := db.GetGame2(id)
		game.MoveList = nil
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB Error: " + err.Error()})
			return
		}

		if game.ID == "" {
			c.JSON(http.StatusNotFound, gin.H{"message": fmt.Sprintf("no game found for id %s", id)})
			return
		}

		c.JSON(http.StatusOK, game)
	})

	r.GET("/games2", func(c *gin.Context) {
		playerID := c.Query("playerId")
		createdAtStr := c.Query("createdAt")
		if playerID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "queries required: ?playerId=<playerId>&createdAt=<timestamp>"})
			return
		}

		if createdAtStr == "" {
			createdAtStr = "0"
		}

		createdAt, err := strconv.ParseInt(createdAtStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unable to parse created at parameter"})
			return
		}

		gameStream, err := db.GetGames(playerID, createdAt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB Error: " + err.Error()})
			return
		}

		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Flush()

		for game := range gameStream {
			gameJSON, _ := json.Marshal(game)
			c.Writer.Write([]byte("data: " + string(gameJSON) + "\n\n"))
			c.Writer.Flush()
		}

		// Send a final event to indicate the end of the stream
		c.Writer.Write([]byte("event: done\ndata: Stream complete\n\n"))
		c.Writer.Flush()

	})

	r.GET("/streams/count", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"count": events.Pub.GetSubCount()})
	})

	r.GET("/streams/:ids", func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("Transfer-Encoding", "chunked")
		c.Writer.Flush()

		// this needs validation of gameIDs
		ids := c.Param("ids")
		gameIDs := strings.Split(ids, ",")
		gameStream := events.NewSubscription(gameIDs)
		defer gameStream.Destroy()

		clientClosed := c.Writer.CloseNotify()

		// publish the initial states of the games to the new stream
		go func() {
			for _, id := range gameIDs {
				game, err := db.GetGame2(id)
				if err != nil {
					continue
				}
				gameUpdate := games.GetGameUpdate(game)
				jsonData, _ := json.Marshal(gameUpdate)
				gameStream.PublishMessage(string(jsonData))

			}
		}()

		for {
			select {
			case <-clientClosed:
				fmt.Println("client dropped SSE")
				return
			case gameData := <-gameStream.Channel:
				c.Writer.Write([]byte("event: gameUpdate\n"))
				c.Writer.Write([]byte("data: " + gameData + "\n\n"))
				c.Writer.Flush()
			}
		}

	})

	// pingTicker := time.NewTicker(1 * time.Second)
	// defer pingTicker.Stop()
	// case <-pingTicker.C:
	// 	c.Writer.Write([]byte("event: ping\n"))
	// 	c.Writer.Write([]byte("data: \n\n"))
	// 	c.Writer.Flush()

	r.Use(PullToken())

	// get a yow jwt token by sending a lichess token
	r.GET("/token", func(c *gin.Context) {
		lichessToken := c.GetString("token")

		tokenRes, err := auth.GetToken(lichessToken)
		if err != nil {
			switch err.(type) {
			case *auth.AuthError:
				c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
				return
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}

		c.JSON(http.StatusOK, tokenRes)
	})

	//......................................
	// ..... Authed routes start here......
	//.....................................

	r.Use(Auth())

	r.POST("/games2", func(c *gin.Context) {
		var newGame models.Game2New

		// Binding and validation
		if err := c.ShouldBindJSON(&newGame); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		now := time.Now().UnixMilli()
		id, _ := games.GetGameID()

		game := models.Game2{
			ID:            id,
			LichessID:     "",
			CreatedAt:     now,
			LastMoveAt:    now,
			Winner:        "pending",
			Method:        "",
			Moves:         "",
			MoveList:      []string{},
			WhiteWillDraw: false,
			BlackWillDraw: false,
			WhitePlayer:   newGame.WhitePlayer,
			BlackPlayer:   newGame.BlackPlayer,
		}

		err := checkHasValidCMP(game)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		user, err := GetUser(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ok := gameHasUser(game, user)
		if !ok {
			c.JSON(http.StatusUnauthorized,
				gin.H{"error": fmt.Sprintf("only authororized to make games for %s", user)})
			return
		}

		result, err := db.CreateGame2(game)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "DB Error: " + err.Error()})
			return
		}

		// MongoDB already had this ID in the DB, so it didn't create a new one
		if result.MatchedCount > 0 {
			c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("game %s already exist, no new creation", game.ID)})
			return
		}

		c.JSON(http.StatusOK, game)

		games.PublishGameUpdates(game.ID)
	})

	r.POST("/games2/:id/moves", func(c *gin.Context) {
		id := c.Param("id")

		var moveData models.MoveData2
		if err := c.ShouldBindJSON(&moveData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		user, err := GetUser(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		err = games.AddMove(id, user, moveData)
		if err != nil {
			httpErr, ok := err.(*utils.HTTPError)
			if ok {
				c.JSON(httpErr.StatusCode, gin.H{"error": httpErr.Message})
				return
			}

			// for some reason we weren't able to type cast to the HTTPError type
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": "move successfully added"})
	})

	r.POST("/games2/:id/:color/draw", func(c *gin.Context) {
		id := c.Param("id")
		color := c.Param("color")
		// not proper route, color should be black or white
		if color != "white" && color != "black" {
			c.JSON(http.StatusNotFound, "")
			return
		}

		userID, err := GetUser(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		err = games.OfferDraw(id, userID, color)
		if err != nil {
			httpErr, ok := err.(*utils.HTTPError)
			if ok {
				c.JSON(httpErr.StatusCode, gin.H{"error": httpErr.Message})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "draw offer made"})
	})

	r.DELETE("/games2/:id/:color/draw", func(c *gin.Context) {
		id := c.Param("id")
		color := c.Param("color")
		// not proper route, color should be black or white
		if color != "white" && color != "black" {
			c.JSON(http.StatusNotFound, "")
			return
		}

		userID, err := GetUser(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		err = games.ClearDrawOffer(id, userID, color)
		if err != nil {
			httpErr, ok := err.(*utils.HTTPError)
			if ok {
				c.JSON(httpErr.StatusCode, gin.H{"error": httpErr.Message})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "draw offer cleared"})
	})

	r.POST("/games2/:id/:color/resign", func(c *gin.Context) {
		id := c.Param("id")
		color := c.Param("color")
		// not proper route, color should be black or white
		if color != "white" && color != "black" {
			c.JSON(http.StatusNotFound, "")
			return
		}

		userID, err := GetUser(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		err = games.Resign(id, userID, color)
		if err != nil {
			httpErr, ok := err.(*utils.HTTPError)
			if ok {
				c.JSON(httpErr.StatusCode, gin.H{"error": httpErr.Message})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "game resigned"})
	})

	r.POST("/games2/:id/:color/abort", func(c *gin.Context) {
		id := c.Param("id")
		color := c.Param("color")
		// not proper route, color should be black or white
		if color != "white" && color != "black" {
			c.JSON(http.StatusNotFound, "")
			return
		}

		userID, err := GetUser(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		err = games.Abort(id, userID, color)
		if err != nil {
			httpErr, ok := err.(*utils.HTTPError)
			if ok {
				c.JSON(httpErr.StatusCode, gin.H{"error": httpErr.Message})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "game aborted"})
	})

	//......................................
	// ..... Admin routes start here......
	//.....................................

	r.POST("/games2/:id/lichessID", CheckRole("admin"), func(c *gin.Context) {
		id := c.Param("id")

		lichessInfo, err := games.GetLichessInfo(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "getting lichess info: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, lichessInfo)
	})

	r.GET("/users", CheckRole("admin"), func(c *gin.Context) {
		usersResponse, err := db.GetAllUsers()
		if err != nil {
			// Handle the error appropriately
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve users"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"count": usersResponse.Count,
			"ids":   usersResponse.IDs,
		})
	})

	r.DELETE("/users/:id", CheckRole("admin"), func(c *gin.Context) {
		id := c.Param("id")

		result, err := db.DeleteUser(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB Error: " + err.Error()})
			return
		}

		if result.DeletedCount == 0 {
			c.JSON(http.StatusNotFound, gin.H{"message": "No user found with given ID"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
	})

	r.GET("/games", CheckRole("admin"), func(c *gin.Context) {
		userId := c.Query("userId")

		allGames, err := db.GetAllGames(userId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB Error: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, allGames)
	})

	r.DELETE("/games/:id", CheckRole("admin"), func(c *gin.Context) {
		id := c.Param("id")

		result, err := db.DeleteGame(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB Error: " + err.Error()})
			return
		}

		if result.DeletedCount == 0 {
			c.JSON(http.StatusNotFound, gin.H{"message": "game not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "game deleted successfully"})
	})

	r.POST("/games2/historical", CheckRole("admin"), func(c *gin.Context) {
		var game models.Game2

		// Binding and validation
		if err := c.ShouldBindJSON(&game); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		moves := strings.Fields(game.Moves)

		_, err := games.ParseGame(game)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		game.MoveList = moves
		game.Moves = ""

		result, err := db.CreateGame2(game)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "DB Error: " + err.Error()})
			return
		}

		// MongoDB already had this ID in the DB, so it didn't create a new one
		if result.MatchedCount > 0 {
			c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("game %s already exist, no new creation", game.ID)})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": fmt.Sprintf("game %s successfully added", game.ID)})
	})

	r.DELETE("/games2/:id", CheckRole("admin"), func(c *gin.Context) {
		id := c.Param("id")

		result, err := db.DeleteGame2(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB Error: " + err.Error()})
			return
		}

		if result.DeletedCount == 0 {
			c.JSON(http.StatusNotFound, gin.H{"message": "game not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "game deleted successfully"})
	})

	r.POST("/settings", CheckRole("admin"), func(c *gin.Context) {
		var settings models.Settings
		if err := c.ShouldBindJSON(&settings); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "DB Error: " + err.Error()})
			return
		}

		if err := db.UpdateSettings(settings); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB Error: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Settings updated successfully"})
	})

	r.GET("/settings", CheckRole("admin"), func(c *gin.Context) {
		settings, err := db.GetSettings()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB Error: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, settings)
	})

	r.POST("/move-req", CheckRole("admin"), func(c *gin.Context) {
		var moveReq models.MoveReq
		if err := c.ShouldBindJSON(&moveReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		_, ok := cmpMap[moveReq.CmpName]
		if !ok {
			errMsg := fmt.Sprintf("%s is not a valid personality", moveReq.CmpName)
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		moveData, err := moveque.GetMove(moveReq)
		if err != nil {
			fmt.Println("There was ane error getting the move: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{"messagge": "queue error"})
			return
		}

		// worker failed to process move requests, return a 400 error
		if moveData.Err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": *moveData.Err})
			return
		}

		c.JSON(http.StatusOK, moveData)
	})

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"messagge": "404 Not Found"})
	})

	portStr := ":" + port

	if port == "8443" || port == "64355" {
		r.RunTLS(portStr, "./certs/cert.pem", "./certs/key.pem")
		return
	}

	r.Run(portStr)
}

func PullToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "No Authorization header provided",
			})
			c.Abort()
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid Authorization header",
			})
			c.Abort()
			return
		}
		token := parts[1]
		c.Set("token", token)
		c.Next()
	}
}

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.GetString("token")

		claims, err := auth.CheckToken(tokenStr)
		if err != nil {
			switch err.(type) {
			case *auth.AuthError:
				c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
				c.Abort()
				return
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				c.Abort()
				return
			}
		}

		c.Set("roles", claims.Roles)
		c.Set("user", claims.Subject)
		c.Next()
	}
}

func CheckRole(allowedRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roles := c.GetStringSlice("roles")

		for _, role := range roles {
			if role == allowedRole {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusUnauthorized, gin.H{"error": "incorrect role for this action"})
		c.Abort()
	}
}

func GetUser(c *gin.Context) (string, error) {
	u, exists := c.Get("user")
	if !exists {
		return "", fmt.Errorf("user not found in context")
	}

	user, ok := u.(string)
	if !ok {
		return "", fmt.Errorf("user in context is not a string")
	}

	return user, nil
}

func userIsValid(userId string) bool {
	regexPattern := `^[a-zA-Z0-9][a-zA-Z0-9_-]{0,28}[a-zA-Z0-9]$`
	matched, _ := regexp.MatchString(regexPattern, userId)
	return matched
}

func loadCmps() {
	file, err := os.Open("personalities.json")
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&cmpMap)
	if err != nil {
		fmt.Println("Error decoding JSON:", err)
	}
}

func checkHasValidCMP(game models.Game2) error {

	hasCMP := game.WhitePlayer.Type == "cmp" || game.BlackPlayer.Type == "cmp"
	if !hasCMP {
		return fmt.Errorf("at least one player must be type cmp")
	}

	ok := true
	if game.WhitePlayer.Type == "cmp" {
		_, ok = cmpMap[game.WhitePlayer.ID]
	}
	if !ok {
		return fmt.Errorf("%s is not a valid personality", game.WhitePlayer.ID)
	}

	if game.BlackPlayer.Type == "cmp" {
		_, ok = cmpMap[game.BlackPlayer.ID]
	}
	if !ok {
		return fmt.Errorf("%s is not a valid personality", game.BlackPlayer.ID)
	}

	return nil
}

func gameHasUser(game models.Game2, user string) bool {

	// bad form in a function called gameHasuser user but whatev
	if game.WhitePlayer.Type == "cmp" && game.BlackPlayer.Type == "cmp" {
		return true
	}

	if game.WhitePlayer.Type == "lichess" && game.WhitePlayer.ID == user {
		return true
	}
	if game.BlackPlayer.Type == "lichess" && game.BlackPlayer.ID == user {
		return true
	}
	return false
}
