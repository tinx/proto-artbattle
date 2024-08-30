package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/olahol/melody"
	"github.com/tinx/proto-artbattle/database"
)

func main() {
	err := LoadConfiguration()
	if (err != nil) {
		fmt.Fprintf(os.Stderr, "error reading configuration: %v\n", err)
		os.Exit(1)
	}

	db := database.Create()
	err = db.Open(Config.DB)
	if (err != nil) {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}

	err = db.Migrate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error migrating database: %v\n", err)
		os.Exit(1)
	}

	fill_db(db)

	file := "file.txt"

	m := melody.New()
	w, _ := fsnotify.NewWatcher()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		m.HandleRequest(w, r)
	})

	m.HandleConnect(func(s *melody.Session) {
		content, _ := os.ReadFile(file)
		s.Write(content)
	})

	go func() {
		for {
			ev := <-w.Events
			if ev.Op == fsnotify.Write {
				content, _ := os.ReadFile(ev.Name)
				m.Broadcast(content)
				fmt.Println("file change")
			}
		}
	}()

	w.Add(file)

	http.ListenAndServe(fmt.Sprintf(":%d", Config.Port), nil)
}


func fill_db(db *database.MysqlRepository) {
	a, err := db.GetArtworkById(1);
	/*
	a := &database.Artwork{
		Title: "Work 1",
		Artist: "Artist 1",
		EloRating: 800,
		DuelCount: 2,
	}
	db.AddArtwork(a)
	*/
	rank, err := db.GetArtworkRank(a)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting artwork rank: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("rank: %d\n", rank)
	/*
	res, err := db.GetArtworksWithSimilarEloRating(a, 2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting artwork rank: %s\n", err)
		os.Exit(1)
	}
	for _, val := range res {
		fmt.Printf("Title: %s\n", val.Title)
	}
	*/
	res, err := db.GetArtworkWithLowestDuelCount();
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting artwork rank: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("Title: %s\n", res.Title)

}

