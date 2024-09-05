package main

import (
	"fmt"
	"encoding/json"
	"math"
	"math/rand"
	"net/http"
	"os"
	"time"

//	"github.com/fsnotify/fsnotify"
	"github.com/olahol/melody"
	"github.com/tinx/proto-artbattle/database"
	"github.com/tinx/proto-artbattle/imagescan"
	"gorm.io/gorm"
)

type ArtworkDTO struct {
	ID		uint   `json:"id"`
	Title		string `json:"title"`
	Artist		string `json:"artist"`
	Filename	string `json:"filename"`
	Panel		string `json:"panel"`
	EloRating	int16 `json:"elo_rating"`
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

type DecisionDTO struct {
	Red		ArtworkDTO `json:"red"`
	Blue		ArtworkDTO `json:"blue"`
	Winner		string `json:"winner"`
	RedEloDiff	int16 `json:"red_elo_diff"`
	RedRankDiff	int64 `json:"red_rank_diff"`
	BlueEloDiff	int16 `json:"blue_elo_diff"`
	BlueRankDiff	int64 `json:"blue_rank_diff"`
}

type ErrorDTO struct {
	Message		string `json:"message"`
}

type SplashscreenDTO struct {
	DuelCount	int64 `json:"duel_count"`
}

type ButtonDTO struct {
	Button		string `json:"button"`
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

	m := melody.New()
	// w, _ := fsnotify.NewWatcher()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	http.HandleFunc("/images/", func(w http.ResponseWriter, r *http.Request) {
		img := r.URL.Path
		img = img[8:]
		http.ServeFile(w, r, "images/" + img)
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		m.HandleRequest(w, r)
	})

	m.HandleConnect(func(s *melody.Session) {
		/* XXX TODO: send last Broadcast message */
		/*
		content, _ := os.ReadFile(file)
		s.Write(content)
		 */
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
		/* we read up to a kilobyte, but only the last byte matters */
		buf := make([]byte, 1024)
		for {
			count, err := serialPort.Read(buf)
			if err != nil {
				fmt.Fprintf(os.Stderr, "serial read error: %s\n", err)
				os.Exit(1)
			}
			if count > 0 {
				sp <- buf[count-1:count]
			}
		}
	}()

	m.HandleMessage(func(s *melody.Session, msg []byte) {
		txt := string(msg);
		if len(txt) > 8 && txt[:8] == "BUTTON: " {
			var dto ButtonDTO;
			err := json.Unmarshal(msg[8:], &dto)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error unmarshalling button dto: %s\n", err)
				return
			}
			if dto.Button != "b" && dto.Button != "r" {
				fmt.Fprintf(os.Stderr, "unexpected button: %s\n", dto.Button)
				return
			}
			sp <- []byte(dto.Button)
		}
		s.Write([]byte("PONG: "))
	})

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
		var input string
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
				input = waitForSerialPort(sp, 15 * time.Second)
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
				waitForSerialPort(sp, 2 * time.Second)
				state = "Leaderboard"
			case "Leaderboard":
				json, err := getLeaderboard(db)
				if err != nil {
					state = "Error"
					lastError = fmt.Sprintf("imeout errorderboard: %s", err)
					continue
				}
				m.Broadcast([]byte("LEADERBOARD: " + json))
				waitForSerialPort(sp, 15 * time.Second)
				state = "SplashScreen"
			case "SplashScreen":
				json, err := getSplashScreen(db)
				if err != nil {
					state = "Error"
					lastError = fmt.Sprintf("Splash screen error: %s", err)
					continue
				}
				m.Broadcast([]byte("SPLASH: " + json))
				waitForSerialPort(sp, 15 * time.Second)
				state = "Duel"
			case "Decision":
				json, err := processDecision(db, a1, a2, input[0])
				if err != nil {
					state = "Error"
					lastError = fmt.Sprintf("Decision error: %s", err)
					continue
				}
				m.Broadcast([]byte("DECISION: " + json))
				waitForSerialPort(sp, 2 * time.Second)
				state = "Duel"
			case "Error":
				var dto ErrorDTO
				dto.Message = lastError
				json, err := json.Marshal(&dto)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error encoding error message: %s\n", lastError)
					continue
				}
				m.Broadcast([]byte("ERROR: " + string(json)))
				waitForSerialPort(sp, 30 * time.Second)
				state = "Duel"
			default:
				state = "Duel"
			}
		}
	}()

	// w.Add(file)

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
	/* consume left-over data in the channel */
	Loop:
	for {
		select {
		case <-c:
		default:
			break Loop
		}
	}
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
	if len(artworks) < 1 {
		fmt.Fprintf(os.Stderr, "no contenders available\n", err)
		return nil, fmt.Errorf("no contenders available")
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

func getSplashScreen(db *database.MysqlRepository) (string, error) {
	var dto SplashscreenDTO;
	count, err := db.GetTotalDuelCount()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting total duel count: %s\n", err)
		return "", err
	}
	dto.DuelCount = count
	j, err := json.Marshal(dto)
	if err != nil {
		fmt.Fprintf(os.Stderr, "json marhsal error: %s\n", err)
		return "", err
	}
	return string(j), nil
}

func processDecision(db *database.MysqlRepository, a1 *database.Artwork, a2 *database.Artwork, decision byte) (string, error) {
	var dto DecisionDTO
	var winner string
	process_decision := func(tx *gorm.DB) error {
		a1_rank_old, err := database.GetArtworkRank(tx, a1)
		a2_rank_old, err := database.GetArtworkRank(tx, a2)
		var a1ed, a2ed int16
		/* Adjust depending on decision */
		if decision == 'r' {
			a1ed, a2ed = eloRatingAdjustments(a1.EloRating, a2.EloRating)
			winner = "red"
		} else if decision == 'b' {
			a2ed, a1ed = eloRatingAdjustments(a2.EloRating, a1.EloRating)
			winner = "blue"
		} else {
			return fmt.Errorf("unexpected decision: %c\n", decision)
		}

		a1.EloRating = a1.EloRating + a1ed
		a2.EloRating = a2.EloRating + a2ed
		dto.RedEloDiff = a1ed
		dto.BlueEloDiff = a2ed

		a1.DuelCount = a1.DuelCount + 1
		a2.DuelCount = a2.DuelCount + 1

		err = database.UpdateArtwork(tx, a1)
		if err != nil {
			return fmt.Errorf("error updating artwork: %s\n", err)
		}
		err = database.UpdateArtwork(tx, a2)
		if err != nil {
			return fmt.Errorf("error updating artwork: %s\n", err)
		}
		a1_rank_new, err := database.GetArtworkRank(tx, a1)
		a2_rank_new, err := database.GetArtworkRank(tx, a2)

		dto.RedRankDiff = a1_rank_old - a1_rank_new
		dto.BlueRankDiff = a2_rank_old - a2_rank_new
		dto.Winner = winner
		return nil
	}
	err := db.Transaction(process_decision)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error processnig decision: %s\n", err)
		return "", err
	}

	encodeArtworkToDTO(a1, &dto.Red)
	encodeArtworkToDTO(a2, &dto.Blue)

	j, err := json.Marshal(dto)
	if err != nil {
		fmt.Fprintf(os.Stderr, "json marhsal error: %s\n", err)
		return "", err
	}
	return string(j), nil
}

/* Elo rating: the points scored by the winner (and paid for by the loser)
   depend on the rating of the players.  Highly rated players can only gain
   few points from winning against low rated players, but they can lose a
   lot of points. */
func eloRatingAdjustments(elo_winner, elo_loser int16) (int16, int16) {
	/* The K-factor is fixed to 16 for simplicity reasons, but it could
	   depend on the elo ranking of the winner/loser. See Wikipedia
	   on the Elo rating system and k-factor. */
	var k_factor = 16.0

	elo_w := float64(elo_winner)
	elo_l := float64(elo_loser)

	expected_score_loser  := 1.0 / (1.0 + math.Pow(10, float64((elo_l - elo_w)/400.0)))
	elo_diff_loser := k_factor * (1.0 - expected_score_loser)
	if elo_diff_loser <= 1.0 {
		elo_diff_loser = 1.0
	}

	return int16(math.Round(elo_diff_loser)), int16(math.Round(-elo_diff_loser))
}

