package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/exp/rand"
)

var client *mongo.Client

func init() {
	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	username := "pokemon_db"
	password := "H6fdOF2505Qn3boQ"
	uri := fmt.Sprintf("mongodb+srv://%s:%s@clusterpoke.hexkh.mongodb.net/?retryWrites=true&w=majority&appName=ClusterPoke", username, password)

	clientOptions := options.Client().ApplyURI(uri)
	var err error
	client, err = mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	// Verify the connection
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Conectado a MongoDB!")
}

// Structure to store daily Pokemon
type DailyPokemon struct {
	Pokemon bson.M `bson:"pokemon"`
	Date    string `bson:"date"`
	GameID  int    `bson:"game_id"`
}

// Function to get a random Pokemon
func getRandomPokemon() (bson.M, error) {
	collection := client.Database("pokemon_db").Collection("pokemon")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Find the total number of Pokemon in the collection
	count, err := collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	// Generate a random index within the range of IDs
	rand.Seed(uint64(time.Now().UnixNano()))
	randomIndex := rand.Int63n(count)

	// Find the Pokemon with the random index
	var pokemon bson.M
	err = collection.FindOne(ctx, bson.M{"id": randomIndex + 1}).Decode(&pokemon)
	if err != nil {
		return nil, err
	}

	return pokemon, nil
}

// Function to schedule daily Pokemon

func scheduleDailyPokemon() {
	// Wait until 00:00 of the next day
	now := time.Now()
	nextScheduledTime := time.Date(now.Year(), now.Month(), now.Day(), 23, 0, 0, 0, now.Location())
	durationUntilScheduledTime := nextScheduledTime.Sub(now)
	time.Sleep(durationUntilScheduledTime)

	// Generate three random Pokemon and store them
	for gameID := 1; gameID <= 3; gameID++ {

		// Get a random Pokemon
		pokemon, err := getRandomPokemon()
		if err != nil {
			log.Printf("Error obtaining random pokémon: %v", err)
			continue
		}

		// Create a DailyPokemon struct with the random Pokemon, date, and game ID
		dailyPokemon := DailyPokemon{
			Pokemon: pokemon,
			Date:    time.Now().Format(time.RFC3339),
			GameID:  gameID,
		}

		// Store the DailyPokemon in the "daily_pokemon" collection
		dailyPokemonCollection := client.Database("pokemon_db").Collection("daily_pokemon")
		_, err = dailyPokemonCollection.InsertOne(context.Background(), dailyPokemon)
		if err != nil {
			log.Printf("Error saving daily pokémon: %v", err)
		} else {
			log.Printf("New daily pokémon (GameID: %d) saved!", gameID)
		}
	}
}

// Function to handle HTTP request to get all Pokemon
func getPokemons(w http.ResponseWriter, r *http.Request) {
	collection := client.Database("pokemon_db").Collection("pokemon")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var pokemons []bson.M
	if err = cursor.All(ctx, &pokemons); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pokemons)
}

// Function to handle HTTP request to get a Pokemon by ID
func getPokemonByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	// Convertir a int si el ID en MongoDB es numérico
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID must be a number", http.StatusBadRequest)
		return
	}

	collection := client.Database("pokemon_db").Collection("pokemon")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var pokemon bson.M
	err = collection.FindOne(ctx, bson.M{"id": id}).Decode(&pokemon)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pokemon)
}

// Function to handle HTTP request to get a Pokemon by name
func getPokemonByName(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	// Validar que la búsqueda tenga al menos 2 letras
	if len(name) < 2 {
		http.Error(w, "Search must contain at least 2 letters", http.StatusBadRequest)
		return
	}

	collection := client.Database("pokemon_db").Collection("pokemon")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Usar ^ para que el nombre empiece por la cadena dada
	filter := bson.M{"name": bson.M{"$regex": "^" + name, "$options": "i"}}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var pokemons []bson.M
	if err = cursor.All(ctx, &pokemons); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If no results found, 404
	if len(pokemons) == 0 {
		http.Error(w, "No pokémons found with that name", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pokemons)
}

// Function to handle HTTP request to get the evolution chain of a Pokemon by ID
func getEvolutionChainByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID must be a number", http.StatusBadRequest)
		return
	}

	pokemonCollection := client.Database("pokemon_db").Collection("pokemon")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var pokemon bson.M
	err = pokemonCollection.FindOne(ctx, bson.M{"id": id}).Decode(&pokemon)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	evolutionChainID := pokemon["evolution_chain_id"]

	evolutionCollection := client.Database("pokemon_db").Collection("evolution_chain")
	var evolutionChain bson.M
	err = evolutionCollection.FindOne(ctx, bson.M{"id": evolutionChainID}).Decode(&evolutionChain)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(evolutionChain)
}

// Function to handle HTTP request to get the latest daily Pokemon by game ID
func getLatestDailyPokemonByGameID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameIDStr := vars["game_id"]

	// Convertir el game_id a int
	gameID, err := strconv.Atoi(gameIDStr)
	if err != nil {
		http.Error(w, "game_id must be a number", http.StatusBadRequest)
		return
	}

	collection := client.Database("pokemon_db").Collection("daily_pokemon")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"game_id": gameID}
	opts := options.FindOne().SetSort(bson.M{"date": -1})

	var dailyPokemon DailyPokemon
	err = collection.FindOne(ctx, filter, opts).Decode(&dailyPokemon)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dailyPokemon)
}

// Main function to start the server

func main() {
	// Start the daily Pokemon scheduler in a goroutine
	go scheduleDailyPokemon()

	r := mux.NewRouter()

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	r.HandleFunc("/pokemons", getPokemons).Methods("GET")
	r.HandleFunc("/pokemons/{id}", getPokemonByID).Methods("GET")
	r.HandleFunc("/pokemons/name/{name}", getPokemonByName).Methods("GET")
	r.HandleFunc("/pokemons/{id}/evolution", getEvolutionChainByID).Methods("GET")

	r.HandleFunc("/pokemons/daily/{game_id}/latest", getLatestDailyPokemonByGameID).Methods("GET")

	fmt.Println("Servidor iniciado en el puerto 8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
