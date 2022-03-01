package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Stuff struct {
	ID    primitive.ObjectID `bson:"_id"`
	Title string             `bson:"title,omitempty"`
	Body  string             `bson:"body,omitempty"`

	// json keys returns how the json object is marshalled and unmarshalled
	// Body  string             `bson:"body,omitempty" json:"text"`
}

var client *mongo.Client
var collection *mongo.Collection
var ctx = context.TODO()

func mongodbInit() {
	// mongodb stuff
	var err error

	// default mongo db url
	mongoURL := "mongodb://root:root@localhost:27017/"

	// read mongo db url from environment
	if envURL, isSet := os.LookupEnv("MONGODB_URL"); isSet {
		mongoURL = envURL
		log.Print("mongo db url read from environment")
	} else {
		log.Print("using default mongo db url")
	}

	client, err = mongo.NewClient(options.Client().ApplyURI(mongoURL))
	if err != nil {
		log.Fatal(err)
	}
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	collection = client.Database("test").Collection("posts")
}

func main() {
	mongodbInit()
	defer client.Disconnect(ctx)

	configureRoutes()
}

func configureRoutes() {
	router := mux.NewRouter()
	router.HandleFunc("/test", TestHandler)

	router.HandleFunc("/stuff", GetAllStuff).Methods("GET")
	router.HandleFunc("/stuff", SaveStuff).Methods("POST")
	router.HandleFunc("/stuff/{id}", GetStuffById).Methods("GET")
	router.HandleFunc("/stuff/{id}", DeleteStuffById).Methods("DELETE")

	router.Use(routerMiddleware)

	srv := &http.Server{
		Handler: removeTrailingSlash(router),
		Addr:    "0.0.0.0:8080",
	}
	srv.ListenAndServe()
}

// middleware to set content type for every request.
// Otherwise we have tow rite this on every route handler:
//   w.Header().Set("Content-Type", "application/json")
func routerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// because mux treats "/stuff" and "/stuff/" as different routes,
// we remove the trailing slash ourselves
func removeTrailingSlash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimSuffix(r.URL.Path, "/")
		next.ServeHTTP(w, r)
	})
}

// return all objects
func GetAllStuff(w http.ResponseWriter, _ *http.Request) {
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		log.Print(err)
	}
	var allStuff []Stuff
	if err = cursor.All(ctx, &allStuff); err != nil {
		log.Print(err)
	}

	allStuffJson, _ := json.Marshal(allStuff)
	fmt.Fprint(w, string(allStuffJson))
}

// return object by id
func GetStuffById(w http.ResponseWriter, r *http.Request) {
	// get param from url
	params := mux.Vars(r)
	id := params["id"]

	// build primitive to use in query filter
	idPrimitive, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		log.Print(err)
		return
	}

	// query db
	result := collection.FindOne(ctx, bson.D{{"_id", idPrimitive}})

	if result.Err() == mongo.ErrNoDocuments {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// decode into object of desired type
	var decoded Stuff
	err = result.Decode(&decoded)

	if err != nil {
		log.Print(err)
	}

	// marshal and return
	marshalled, _ := json.Marshal(decoded)
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(marshalled))
}

func SaveStuff(w http.ResponseWriter, r *http.Request) {
	// decode body into object
	var decoded Stuff
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&decoded)

	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// set object id
	decoded.ID = primitive.NewObjectID()

	// insert object
	result, err := collection.InsertOne(ctx, decoded)
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// get inserted object id
	id := result.InsertedID.(primitive.ObjectID)
	idBytes, err := id.MarshalJSON()
	if err != nil {
		log.Print(err)
		return
	}

	idString := strings.Trim(string(idBytes), "\"")

	// primitive.NewObjectID().String()
	// set location header
	location := r.Host + "/stuff/" + idString
	w.Header().Set("location", location)

	w.WriteHeader(http.StatusCreated)

	// marshal and return
	marshalled, _ := json.Marshal(decoded)
	fmt.Fprint(w, string(marshalled))
}

// delete object by id
func DeleteStuffById(w http.ResponseWriter, r *http.Request) {
	// get param from url
	params := mux.Vars(r)
	id := params["id"]

	// build primitive to use in query filter
	idPrimitive, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// query db
	result, err := collection.DeleteOne(ctx, bson.D{{"_id", idPrimitive}})
	if err != nil {
		log.Print(err)
		return
	}

	// set header to not found if nothing was deleted
	if result.DeletedCount == 0 {
		w.WriteHeader(http.StatusNotFound)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// test function
func TestHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "{\"status\": \"ok\"}")
}
