package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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
	// // Cargar las variables de entorno desde el archivo .env
	// err := godotenv.Load() // Aquí declaras `err` por primera vez
	// if err != nil {
	// 	log.Fatal("Error loading .env file")
	// }

	// Conectar a MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	uri := os.Getenv("MONGO_URI")

	clientOptions := options.Client().ApplyURI(uri)
	var connErr error // Declaras una nueva variable `connErr` para manejar la conexión a MongoDB
	client, connErr = mongo.Connect(ctx, clientOptions)
	if connErr != nil {
		log.Fatal(connErr)
	}

	// Verificar la conexión
	err := client.Ping(ctx, nil) // Usas la variable `err` ya declarada
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

	// Find the total number of Pokemon in the collection for both ID ranges (1-1025 and 10001-10279)
	count1, err := collection.CountDocuments(ctx, bson.M{"id": bson.M{"$gte": 1, "$lte": 1025}})
	if err != nil {
		return nil, err
	}
	count2, err := collection.CountDocuments(ctx, bson.M{"id": bson.M{"$gte": 10001, "$lte": 10279}})
	if err != nil {
		return nil, err
	}

	totalCount := count1 + count2

	// Generate a random index within the total count
	rand.Seed(uint64(time.Now().UnixNano()))
	randomIndex := rand.Int63n(int64(totalCount))

	var pokemon bson.M
	if randomIndex < int64(count1) {
		// Select from the first range (1-1025)
		err = collection.FindOne(ctx, bson.M{"id": randomIndex + 1}).Decode(&pokemon)
	} else {
		// Select from the second range (10001-10279)
		err = collection.FindOne(ctx, bson.M{"id": randomIndex - int64(count1) + 10001}).Decode(&pokemon)
	}
	if err != nil {
		return nil, err
	}

	return pokemon, nil
}
func scheduleDailyPokemon() {
	for {
		// Calcular la próxima ejecución a las 00:00 del siguiente día
		now := time.Now()
		nextScheduledTime := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		if now.After(nextScheduledTime) {
			nextScheduledTime = nextScheduledTime.Add(24 * time.Hour)
		}
		durationUntilScheduledTime := nextScheduledTime.Sub(now)
		log.Printf("Time until next daily Pokémon: %v", durationUntilScheduledTime)
		time.Sleep(durationUntilScheduledTime)

		// Generar los 3 Pokémon diarios
		for gameID := 1; gameID <= 3; gameID++ {
			pokemon, err := getRandomPokemon()
			if err != nil {
				log.Printf("Error obtaining random Pokémon: %v", err)
				continue
			}

			dailyPokemon := DailyPokemon{
				Pokemon: pokemon,
				Date:    time.Now().Format(time.RFC3339),
				GameID:  gameID,
			}

			dailyPokemonCollection := client.Database("pokemon_db").Collection("daily_pokemon")
			_, err = dailyPokemonCollection.InsertOne(context.Background(), dailyPokemon)
			if err != nil {
				log.Printf("Error saving daily Pokémon: %v", err)
			} else {
				log.Printf("New daily Pokémon (GameID: %d) saved!", gameID)
			}
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

	if len(name) < 2 {
		http.Error(w, "Search must contain at least 2 letters", http.StatusBadRequest)
		return
	}

	collection := client.Database("pokemon_db").Collection("pokemon")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"name": bson.M{"$eq": name}}

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

// Function to handle HTTP request to get the latest daily Pokemon by game ID
func getLatestDailyPokemonByGameID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameIDStr := vars["game_id"]

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

	var dailyPokemon bson.M
	err = collection.FindOne(ctx, filter, opts).Decode(&dailyPokemon)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dailyPokemon)
}

// Function to handle HTTP request to get the yesterday daily Pokemon by game ID
func getSecondLatestDailyPokemonByGameID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameIDStr := vars["game_id"]

	gameID, err := strconv.Atoi(gameIDStr)
	if err != nil {
		http.Error(w, "game_id must be a number", http.StatusBadRequest)
		return
	}

	collection := client.Database("pokemon_db").Collection("daily_pokemon")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Buscar los dos últimos registros ordenados por fecha en orden descendente
	filter := bson.M{"game_id": gameID}
	opts := options.Find().SetSort(bson.M{"date": -1}).SetLimit(2)

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var dailyPokemons []bson.M
	if err = cursor.All(ctx, &dailyPokemons); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Verificar que hay al menos dos resultados
	if len(dailyPokemons) < 2 {
		http.Error(w, "Not enough records found", http.StatusNotFound)
		return
	}

	// Devolver el segundo más reciente
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dailyPokemons[1])
}

// Function to handle HTTP request to add 3 new DailyPokemon
func addThreeDailyPokemons(w http.ResponseWriter, r *http.Request) {
	// Generate three random Pokemon and store them
	for gameID := 1; gameID <= 3; gameID++ {

		// Get a random Pokemon
		pokemon, err := getRandomPokemon()
		if err != nil {
			log.Printf("Error obtaining random pokémon: %v", err)
			http.Error(w, fmt.Sprintf("Error obtaining random pokémon: %v", err), http.StatusInternalServerError)
			return
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
			http.Error(w, fmt.Sprintf("Error saving daily pokémon: %v", err), http.StatusInternalServerError)
			return
		} else {
			log.Printf("New daily pokémon (GameID: %d) saved!", gameID)
		}
	}

	// Respond with a success message
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "3 new daily Pokémon added successfully"})
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // Permitir cualquier origen
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// If it is a preflight request (OPTIONS), 200 OK
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Main function to start the server
func main() {

	// Start the daily Pokemon scheduler in a goroutine
	go scheduleDailyPokemon()

	r := mux.NewRouter()
	r.Use(enableCORS)

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		fmt.Println("Hey, I'm still running")
	}).Methods("GET")

	r.HandleFunc("/pokemons", getPokemons).Methods("GET")
	r.HandleFunc("/pokemons/{id}", getPokemonByID).Methods("GET")
	r.HandleFunc("/pokemons/name/{name}", getPokemonByName).Methods("GET")

	r.HandleFunc("/pokemons/daily/{game_id}/latest", getLatestDailyPokemonByGameID).Methods("GET")
	r.HandleFunc("/pokemons/daily/{game_id}/yesterday", getSecondLatestDailyPokemonByGameID).Methods("GET")

	r.HandleFunc("/pokemons/daily/add", addThreeDailyPokemons).Methods("POST")

	fmt.Println("Server initialized in port: 8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
