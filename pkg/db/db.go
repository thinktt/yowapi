package db

import (
	"context"
	"fmt"
	"os"

	"github.com/thinktt/yowapi/pkg/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var client *mongo.Client
var yowDatabase *mongo.Database

// UserIDsResponse represents the structure of the response
type UserIDsResponse struct {
	Count int      `json:"count"`
	IDs   []string `json:"ids"`
}

type AllGames struct {
	Count   int      `json:"count"`
	GameIDs []string `json:"ids"`
}

func init() {
	// Connect to MongoDB
	mongoHost := os.Getenv("MONGO_HOST")
	if mongoHost == "" {
		mongoHost = "localhost:27017"
	}
	uri := fmt.Sprintf("mongodb://%s/?maxPoolSize=20&w=majority", mongoHost)
	var err error
	clientOptions := options.Client().ApplyURI(uri)
	client, err = mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		panic(err)
	}

	yowDatabase = client.Database("yow")

	// make sure games2 collection has unique index for its ids
	gamesCollection := yowDatabase.Collection("games2")
	indexModel := mongo.IndexModel{
		Keys:    bson.M{"id": 1}, // Index on the `id` field
		Options: options.Index().SetUnique(true),
	}
	_, err = gamesCollection.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		panic(err)
	}

}

func CreateUser(user models.User) (*mongo.UpdateResult, error) {
	usersCollection := yowDatabase.Collection("users")

	// Manually create a bson.M map from the User struct
	userMap := bson.M{
		"id":                    user.ID,
		"kingCmVersion":         user.KingCmVersion,
		"hasAcceptedDisclaimer": user.HasAcceptedDisclaimer,
	}

	filter := bson.M{"id": userMap["id"]}
	update := bson.M{"$setOnInsert": userMap}
	options := options.Update().SetUpsert(true)

	return usersCollection.UpdateOne(context.Background(), filter, update, options)
}

func GetUser(id string) (models.User, error) {
	usersCollection := yowDatabase.Collection("users")

	filter := bson.M{"id": id}
	var user models.User
	err := usersCollection.FindOne(context.Background(), filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return models.User{}, nil // No user found, return empty struct without error
		}
		return models.User{}, err // An error occurred during the query
	}

	return user, nil
}

// GetAllUsers retrieves all user IDs
func GetAllUsers() (UserIDsResponse, error) {
	usersCollection := yowDatabase.Collection("users")

	// Define the projection to include only the id field
	projection := bson.M{"id": 1, "_id": 0}

	// Find all documents with the specified projection
	cursor, err := usersCollection.Find(
		context.Background(),
		bson.M{}, options.Find().SetProjection(projection),
	)
	if err != nil {
		return UserIDsResponse{}, err
	}
	defer cursor.Close(context.Background())

	var results []bson.M
	if err = cursor.All(context.Background(), &results); err != nil {
		return UserIDsResponse{}, err
	}

	// Extract IDs from results
	var ids []string
	for _, result := range results {
		if id, ok := result["id"].(string); ok {
			ids = append(ids, id)
		}
	}

	return UserIDsResponse{
		Count: len(ids),
		IDs:   ids,
	}, nil
}

func DeleteUser(id string) (*mongo.DeleteResult, error) {
	usersCollection := yowDatabase.Collection("users")

	filter := bson.M{"id": id}
	return usersCollection.DeleteOne(context.Background(), filter)
}

// CreateGame creates or updates a game entry in the database
func CreateGame(game models.Game) (*mongo.UpdateResult, error) {
	gamesCollection := yowDatabase.Collection("games")

	filter := bson.M{"id": game.ID}
	update := bson.M{"$setOnInsert": game}
	upsert := true
	opts := options.UpdateOptions{Upsert: &upsert}

	return gamesCollection.UpdateOne(context.Background(), filter, update, &opts)
}

// GetGame retrieves a game by its ID
func GetGame(id string) (bson.M, error) {
	gamesCollection := yowDatabase.Collection("games")

	filter := bson.M{"id": id}
	var result bson.M
	err := gamesCollection.FindOne(context.Background(), filter).Decode(&result)

	// No game found, return nil without error
	if err != nil && err == mongo.ErrNoDocuments {
		return nil, nil
	}

	// return any other errors
	if err != nil {
		return nil, err
	}

	// Remove the MongoDB _id field
	delete(result, "_id")

	return result, nil
}

func GetAllGames(userId string) (AllGames, error) {
	gamesCollection := yowDatabase.Collection("games")

	var filter bson.M
	if userId == "" {
		filter = bson.M{}
	} else {
		filter = bson.M{"user": userId}
	}

	count, err := gamesCollection.CountDocuments(context.Background(), filter)
	if err != nil {
		return AllGames{}, err
	}

	// Define the projection to include only the id field
	projection := bson.M{"id": 1, "_id": 0}

	// Retrieve the first 1000 game IDs
	cursor, err := gamesCollection.Find(
		context.Background(),
		filter,
		options.Find().SetLimit(1000).SetProjection(projection),
	)
	if err != nil {
		return AllGames{}, err
	}
	defer cursor.Close(context.Background())

	var results []bson.M
	if err = cursor.All(context.Background(), &results); err != nil {
		return AllGames{}, err
	}

	// Extract IDs from results
	var ids []string
	for _, result := range results {
		if id, ok := result["id"].(string); ok {
			ids = append(ids, id)
		}
	}

	return AllGames{
		Count:   int(count),
		GameIDs: ids,
	}, nil
}

func DeleteGame(id string) (*mongo.DeleteResult, error) {
	gamesCollection := yowDatabase.Collection("games")

	filter := bson.M{"id": id}
	return gamesCollection.DeleteOne(context.Background(), filter)
}

func UpdateSettings(settings models.Settings) error {
	settingsCollection := yowDatabase.Collection("settings")

	_, err := settingsCollection.UpdateOne(
		context.Background(),
		bson.M{}, // Empty filter matches any document
		bson.M{"$set": settings},
		options.Update().SetUpsert(true), // Upsert: true creates a new document if none exists
	)
	return err
}

func GetSettings() (models.Settings, error) {
	settingsCollection := yowDatabase.Collection("settings")

	var settings models.Settings
	err := settingsCollection.FindOne(context.Background(), bson.M{}).Decode(&settings)
	if err != nil {
		return models.Settings{}, err
	}

	return settings, nil
}
