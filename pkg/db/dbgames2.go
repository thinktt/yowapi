package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/thinktt/yowapi/pkg/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CreateGame creates or updates a game entry in the database
func CreateGame2(game models.Game2) (*mongo.UpdateResult, error) {
	gamesCollection := yowDatabase.Collection("games2")

	filter := bson.M{"id": game.ID}
	update := bson.M{"$setOnInsert": game}
	upsert := true
	opts := options.UpdateOptions{Upsert: &upsert}

	return gamesCollection.UpdateOne(context.Background(), filter, update, &opts)
}

// GetAllLiveGameIDs find sall the games in the database that have game.Winner
// pending and returns a list of all of their IDs
func GetAllLiveGameIDs() ([]string, error) {
	gamesCollection := yowDatabase.Collection("games2")

	// Define the filter to find games with status "started"
	filter := bson.M{"winner": "pending"}

	// Define a projection to return only the "id" field
	projection := bson.M{"id": 1}

	// Find all matching games
	cursor, err := gamesCollection.Find(context.Background(), filter, options.Find().SetProjection(projection))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	// Collect all game IDs
	var gameIDs []string
	for cursor.Next(context.Background()) {
		var result struct {
			ID string `bson:"id"`
		}
		if err := cursor.Decode(&result); err != nil {
			return nil, err
		}
		gameIDs = append(gameIDs, result.ID)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return gameIDs, nil
}

// GetGames takes a user and a createAt time, find all games with ghat user
// after the createAt time, send them as a stream of games objects
func GetGames(playerID string, createdAt int64) (<-chan models.Game2, error) {
	gamesCollection := yowDatabase.Collection("games2")

	filter := bson.M{
		"$or": []bson.M{
			{"whitePlayer.id": playerID},
			{"blackPlayer.id": playerID},
		},
		"createdAt": bson.M{"$gt": createdAt},
	}

	cursor, err := gamesCollection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}

	ch := make(chan models.Game2)

	go func() {
		defer close(ch)
		defer cursor.Close(context.Background())

		for cursor.Next(context.Background()) {
			var game models.Game2
			if err := cursor.Decode(&game); err != nil {
				continue
			}
			game.Moves = strings.Join(game.MoveList, " ")
			game.MoveList = nil
			ch <- game
		}
	}()

	return ch, nil
}

// GetGame retrieves a game by its ID
func GetGame2(id string) (models.Game2, error) {
	gamesCollection := yowDatabase.Collection("games2")

	filter := bson.M{"id": id}
	// var result bson.M
	var game models.Game2
	err := gamesCollection.FindOne(context.Background(), filter).Decode(&game)

	// No game found, return nil without error
	if err != nil && err == mongo.ErrNoDocuments {
		return models.Game2{}, nil
	}

	// return any other errors
	if err != nil {
		return models.Game2{}, err
	}

	// // Remove the MongoDB _id field
	// delete(result, "_id")

	game.Moves = strings.Join(game.MoveList, " ")
	// game.MoveList = nil

	return game, nil
}

func DeleteGame2(id string) (*mongo.DeleteResult, error) {
	gamesCollection := yowDatabase.Collection("games2")

	filter := bson.M{"id": id}
	return gamesCollection.DeleteOne(context.Background(), filter)
}

func CreateMove(gameID, move, userColor string) (*mongo.UpdateResult, error) {
	gamesCollection := yowDatabase.Collection("games2")

	// when we create a move we also clear any draws offers of the opponent
	opponentDrawField := "whiteWillDraw"
	if userColor == "white" {
		opponentDrawField = "blackWillDraw"
	}

	filter := bson.M{"id": gameID}
	update := bson.M{
		"$push": bson.M{
			"moveList": move,
		},
		"$set": bson.M{
			"lastMoveAt":      time.Now().UnixMilli(),
			opponentDrawField: false,
		},
	}

	return gamesCollection.UpdateOne(context.Background(), filter, update)
}

func UpdateGame(gameID, move, winner, method string) (*mongo.UpdateResult, error) {
	gamesCollection := yowDatabase.Collection("games2")

	filter := bson.M{"id": gameID}
	update := bson.M{
		"$push": bson.M{
			"moveList": move,
		},
		"$set": bson.M{
			"lastMoveAt": time.Now().UnixMilli(),
			"winner":     winner,
			"method":     method,
		},
	}

	return gamesCollection.UpdateOne(context.Background(), filter, update)
}

func UpdateWillDraw(gameID, color string, state bool) (*mongo.UpdateResult, error) {
	gamesCollection := yowDatabase.Collection("games2")

	filter := bson.M{"id": gameID}
	updateField := ""
	if color == "white" {
		updateField = "whiteWillDraw"
	} else if color == "black" {
		updateField = "blackWillDraw"
	} else {
		return nil, fmt.Errorf("invalid color specified")
	}

	update := bson.M{
		"$set": bson.M{
			updateField: state,
		},
	}

	return gamesCollection.UpdateOne(context.Background(), filter, update)
}

func DrawGame(gameID string, method string) (*mongo.UpdateResult, error) {
	gamesCollection := yowDatabase.Collection("games2")

	filter := bson.M{"id": gameID}
	update := bson.M{
		"$set": bson.M{
			"winner": "draw",
			"method": method,
		},
	}

	return gamesCollection.UpdateOne(context.Background(), filter, update)
}

func ResignGame(gameID, userColor string) (*mongo.UpdateResult, error) {
	gamesCollection := yowDatabase.Collection("games2")

	filter := bson.M{"id": gameID}
	var winner string
	if userColor == "white" {
		winner = "black"
	} else if userColor == "black" {
		winner = "white"
	} else {
		return nil, fmt.Errorf("invalid color specified")
	}

	update := bson.M{
		"$set": bson.M{
			"winner": winner,
			"method": "resign",
		},
	}

	return gamesCollection.UpdateOne(context.Background(), filter, update)
}

// GetGameIDs retrieves a comma-separated string of game IDs for a given user
// for games created since the specified timestamp
func GetGameIDs(user string, createdAt int64) (map[string]string, error) {
	gamesCollection := yowDatabase.Collection("games2")

	// Define filter to match games where the user is either the white or black
	// player, and the createdAt timestamp is greater than or equal to the
	// provided value
	filter := bson.M{
		"createdAt": bson.M{"$gte": createdAt},
		"$or": []bson.M{
			{"whitePlayer.id": user},
			{"blackPlayer.id": user},
		},
	}

	// Projection to only include the ID field in the results
	projection := bson.M{
		"id": 1,
	}

	cursor, err := gamesCollection.Find(context.Background(), filter, options.Find().SetProjection(projection))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var ids []string
	for cursor.Next(context.Background()) {
		var result struct {
			ID string `bson:"id"`
		}
		if err := cursor.Decode(&result); err != nil {
			return nil, err
		}
		ids = append(ids, result.ID)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return map[string]string{"ids": strings.Join(ids, ",")}, nil
}

func UpdateLichessID(gameID, lichessID string) (*mongo.UpdateResult, error) {
	gamesCollection := yowDatabase.Collection("games2")

	filter := bson.M{"id": gameID}
	update := bson.M{
		"$set": bson.M{
			"lichessId": lichessID,
		},
	}

	return gamesCollection.UpdateOne(context.Background(), filter, update)
}
