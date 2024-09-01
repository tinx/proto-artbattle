package main

import (
	"fmt"
	"encoding/json"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/olahol/melody"
	"github.com/tinx/proto-artbattle/database"
	"github.com/tinx/proto-artbattle/imagescan"
)

type ArtworkDTO struct {
	ID		uint   `json:"id"`
	Title		string `json:"title"`
	Artist		string `json:"artist"`
	Filename	string `json:"filename"`
	Panel		string `json:"panel"`
	EloRating	uint16 `json:"elo_rating"`
	DuelCount	uint64 `json:"duel_count"`
}

type DuelDTO struct {
	Red		ArtworkDTO `json:"red"`
	Blue		ArtworkDTO `json:"blue"`

}

type LeaderboardDTO struct {
	Count		int `json:"count"`
	Entries		[]ArtworkDTO `json:"entries"`
}

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

	imagescan.Scan(Config.ImagePath)

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

	serialPort, err := os.Open("/dev/pts/5")
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't open serial port: %s\n", err)
		os.Exit(1)
	}
	defer serialPort.Close()

	sp := make(chan []byte, 1)

	/* send all serial port input into channel "sp" so we
	   can select() from it. */
	go func() {
		buf := make([]byte, 1)
		for {
			count, err := serialPort.Read(buf)
			if err != nil {
				fmt.Fprintf(os.Stderr, "serial read error: %s\n", err)
				os.Exit(1)
			}
			if count > 0 {
				sp <- buf
			}
		}
	}()

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

	go func() {
		/* Finite State Machine
		 *  Start -> Duel
		 *  Duel -> Timeout
		 *  Duel -> Decision
		 *  Decision -> Duel
		 *  Timeout -> Leaderboard
		 *  Leaderboard -> SplashScreen
		 *  SplashScreen -> Duel
		 *  * -> Error
		 *  Error -> Duel
		 */
		var state string = "Start"
		var lastError = ""
		var a1, a2 *database.Artwork
		for {
			switch state {
			case "Start":
				state = "Duel"
			case "Duel":
				a1, a2, err = generateDuel(db)
				if err != nil {
					state = "Error"
					lastError = fmt.Sprintf("Duel error: %s", err)
					continue
				}
				json, err := encodeDuelToJson(a1, a2)
				if err != nil {
					state = "Error"
					lastError = fmt.Sprintf("Duel error: %s", err)
					continue
				}
				m.Broadcast([]byte("DUEL: " + json))
				input := waitForSerialPort(sp, 10 * time.Second)
				if input == "" {
					state = "Timeout"
				} else {
					state = "Decision"
				}
			case "Timeout":
				json, err := encodeDuelToJson(a1, a2)
				if err != nil {
					state = "Error"
					lastError = fmt.Sprintf("timeout error: %s", err)
					continue
				}
				m.Broadcast([]byte("TIMEOUT: " + json))
				waitForSerialPort(sp, 3 * time.Second)
				state = "Leaderboard"
			case "Leaderboard":
				json, err := getLeaderboard(db)
				if err != nil {
					state = "Error"
					lastError = fmt.Sprintf("imeout errorderboard: %s", err)
					continue
				}
				m.Broadcast([]byte("LEADERBOARD: " + json))
				waitForSerialPort(sp, 5 * time.Second)
				state = "SplashScreen"
			case "SplashScreen":
				m.Broadcast([]byte("SplashScreen"))
				waitForSerialPort(sp, 5 * time.Second)
				state = "Duel"
			case "Decision":
				m.Broadcast([]byte("Decision"))
				waitForSerialPort(sp, 5 * time.Second)
				state = "Duel"
			case "Error":
				m.Broadcast([]byte("Error: " + lastError))
				waitForSerialPort(sp, 30 * time.Second)
				state = "Duel"
			default:
				state = "Duel"
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

func waitForSerialPort(c chan []byte, timeout time.Duration) string {
	select {
	case ret := <-c:
		return string(ret)
	case <-time.After(timeout):
		return ""
	}
	return ""
}

func generateDuel(db *database.MysqlRepository) (*database.Artwork, *database.Artwork, error) {
	a1, err := db.GetArtworkWithLowestDuelCount()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading artwork: %s\n", err)
		return nil, nil, err
	}
	a2, err := getDuelPartner(db, a1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting duel partner: %s\n", err)
		return nil, nil, err
	}
	return a1, a2, nil
}

func getDuelPartner(db *database.MysqlRepository, a *database.Artwork) (*database.Artwork, error) {
	/* get 50 possible contenders with similar Elo rating */
	artworks, err := db.GetArtworksWithSimilarEloRating(a, 50)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error with contender list: %s\n", err)
		return nil, err
	}
	/* version 1: just return a random element */
	return artworks[rand.Intn(len(artworks))], nil
}

func encodeArtworkToDTO(a *database.Artwork, dto *ArtworkDTO) {
	dto.ID = a.ID
	dto.Title = a.Title
	dto.Artist = a.Artist
	dto.Filename = a.Filename
	dto.Panel = a.Panel
	dto.EloRating = a.EloRating
	dto.DuelCount = a.DuelCount
}

func encodeDuelToJson(a1, a2 *database.Artwork) (string, error) {
	var dto DuelDTO
	encodeArtworkToDTO(a1, &dto.Red)
	encodeArtworkToDTO(a2, &dto.Blue)
	j, err := json.Marshal(dto)
	if err != nil {
		fmt.Fprintf(os.Stderr, "json marhsal error: %s\n", err)
		return "", err
	}
	return string(j), nil
}

func getLeaderboard(db *database.MysqlRepository) (string, error) {
	lb, err := db.GetLeaderboard(10)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting leaderboard: %s\n", err)
		return "", err
	}
	dto, err := encodeLeaderboardToDTO(lb)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error encoding leaderboard: %s\n", err)
		return "", err
	}
	j, err := json.Marshal(dto)
	if err != nil {
		fmt.Fprintf(os.Stderr, "json marhsal error: %s\n", err)
		return "", err
	}
	return string(j), nil
}

func encodeLeaderboardToDTO(lb []*database.Artwork) (string, error) {
	var dto LeaderboardDTO
	dto.Count = len(lb)
	for _, a := range lb {
		var aw_dto ArtworkDTO
		encodeArtworkToDTO(a, &aw_dto)
		dto.Entries = append(dto.Entries, aw_dto)
	}
	j, err := json.Marshal(dto)
	if err != nil {
		fmt.Fprintf(os.Stderr, "json marhsal error: %s\n", err)
		return "", err
	}
	return string(j), nil
}
