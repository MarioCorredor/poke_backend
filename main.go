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

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/pokemons", getPokemons).Methods("GET")
	r.HandleFunc("/pokemons/{id}", getPokemonByID).Methods("GET")
	r.HandleFunc("/pokemons/name/{name}", getPokemonByName).Methods("GET")
	r.HandleFunc("/pokemons/{id}/evolution", getEvolutionChainByID).Methods("GET")

	fmt.Println("Servidor iniciado en el puerto 8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
