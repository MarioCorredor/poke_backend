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

// Estructura para almacenar el Pokémon del día
type DailyPokemon struct {
	Pokemon bson.M `bson:"pokemon"`
	Date    string `bson:"date"`
	GameID  int    `bson:"game_id"`
}

// Función para obtener un Pokémon aleatorio
func getRandomPokemon() (bson.M, error) {
	collection := client.Database("pokemon_db").Collection("pokemon")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Encontrar el número total de Pokémon en la colección
	count, err := collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	// Generar un índice aleatorio dentro del rango de IDs
	rand.Seed(uint64(time.Now().UnixNano()))
	randomIndex := rand.Int63n(count)

	// Buscar el Pokémon con el índice aleatorio
	var pokemon bson.M
	err = collection.FindOne(ctx, bson.M{"id": randomIndex + 1}).Decode(&pokemon)
	if err != nil {
		return nil, err
	}

	return pokemon, nil
}

func scheduleDailyPokemon() {
	// Esperar hasta las 14:10 del próximo día
	now := time.Now()
	nextScheduledTime := time.Date(now.Year(), now.Month(), now.Day(), 23, 0, 0, 0, now.Location())
	durationUntilScheduledTime := nextScheduledTime.Sub(now)
	time.Sleep(durationUntilScheduledTime)

	// Generar tres Pokémon aleatorios y almacenarlos
	for gameID := 1; gameID <= 3; gameID++ {
		// Obtener un Pokémon aleatorio
		pokemon, err := getRandomPokemon()
		if err != nil {
			log.Printf("Error al obtener Pokémon aleatorio: %v", err)
			continue
		}

		// Crear la estructura con la fecha, el Pokémon y el game_id
		dailyPokemon := DailyPokemon{
			Pokemon: pokemon,
			Date:    time.Now().Format(time.RFC3339),
			GameID:  gameID,
		}

		// Almacenar el Pokémon en la colección `daily_pokemon`
		dailyPokemonCollection := client.Database("pokemon_db").Collection("daily_pokemon")
		_, err = dailyPokemonCollection.InsertOne(context.Background(), dailyPokemon)
		if err != nil {
			log.Printf("Error al almacenar Pokémon del día: %v", err)
		} else {
			log.Printf("Nuevo Pokémon del día (GameID: %d) almacenado correctamente!", gameID)
		}
	}
}

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

func getPokemonByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	// Convertir a int si el ID en MongoDB es numérico
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID debe ser un número", http.StatusBadRequest)
		return
	}

	collection := client.Database("pokemon_db").Collection("pokemon")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var pokemon bson.M
	err = collection.FindOne(ctx, bson.M{"id": id}).Decode(&pokemon)
	if err != nil {
		print("Entro al nil")
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pokemon)
}

func getPokemonByName(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	// Validar que la búsqueda tenga al menos 2 letras
	if len(name) < 2 {
		http.Error(w, "La búsqueda debe tener al menos 2 letras", http.StatusBadRequest)
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

	// Si no se encuentran resultados, responder con un 404
	if len(pokemons) == 0 {
		http.Error(w, "No se encontraron Pokémon con ese nombre", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pokemons)
}

func getEvolutionChainByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	// Convertir a int si el ID en MongoDB es numérico
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID debe ser un número", http.StatusBadRequest)
		return
	}

	// Primero, obtener el Pokémon para conseguir su evolution_chain_id
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

	// Luego, obtener la cadena de evolución
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

func getLatestDailyPokemonByGameID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameIDStr := vars["game_id"]

	// Convertir el game_id a int
	gameID, err := strconv.Atoi(gameIDStr)
	if err != nil {
		http.Error(w, "game_id debe ser un número", http.StatusBadRequest)
		return
	}

	// Obtener la colección de daily_pokemon
	collection := client.Database("pokemon_db").Collection("daily_pokemon")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Buscar el Pokémon con el game_id más reciente
	filter := bson.M{"game_id": gameID}
	opts := options.FindOne().SetSort(bson.M{"date": -1})

	var dailyPokemon DailyPokemon
	err = collection.FindOne(ctx, filter, opts).Decode(&dailyPokemon)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Devolver el Pokémon más reciente en formato JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dailyPokemon)
}

func main() {
	// Iniciar la rutina diaria en una goroutine
	go scheduleDailyPokemon()

	r := mux.NewRouter()

	r.HandleFunc("/pokemons", getPokemons).Methods("GET")
	r.HandleFunc("/pokemons/{id}", getPokemonByID).Methods("GET")
	r.HandleFunc("/pokemons/name/{name}", getPokemonByName).Methods("GET")
	r.HandleFunc("/pokemons/{id}/evolution", getEvolutionChainByID).Methods("GET")

	r.HandleFunc("/pokemons/daily/{game_id}/latest", getLatestDailyPokemonByGameID).Methods("GET")

	fmt.Println("Servidor iniciado en el puerto 8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
