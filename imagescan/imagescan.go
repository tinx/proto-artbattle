package imagescan

import (
	"path/filepath"
	"fmt"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/tinx/proto-artbattle/database"
	"github.com/tinx/proto-artbattle/internal/repository/config"
)

type ExifData struct {
	SourceFile	string	`json:"SourceFile"`
	UserComment	string	`json:"UserComment"`
}

func Scan(root string) error {
	err := filepath.Walk(root, ScanEntry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "file walk error: %s\n", err)
		return err
	}
	return nil
}

func ScanEntry(path string, info os.FileInfo, err error) error {
	if err != nil {
		fmt.Fprintf(os.Stderr, "file walk error: %s\n", err)
		return err
	}

	if !info.Mode().IsRegular() {
		return nil
	}

	match, _ := regexp.MatchString("^.*\\.(jpg|JPG|jpeg|JPEG)$", path)
	if !match {
		return nil
	}
	match, _ = regexp.MatchString("^.*_tn\\.(jpg|JPG|jpeg|JPEG)$", path)
	if match {
		return nil
	}

	out, err := exec.Command("/usr/bin/exiftool", "-charset", "UTF8", "-j", "-usercomment", path).Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error executing exiftool for %s: %s\n", path, err)
		return nil // not err -> continue with next file
	}

	var exif []ExifData
	err = json.Unmarshal(out, &exif)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing exiftool output for %s: %s\n", path, err)
		return nil // not err -> continue with next file
	}
	if len(exif) != 1 {
		fmt.Fprintf(os.Stderr, "unexpected exif array length for %s: %d\n", path, len(exif))
		return nil // not err -> continue with next file
	}

	usercomment := exif[0].UserComment
	if usercomment == "" {
		fmt.Fprintf(os.Stderr, "missing exif UserComment for %s: %s\n", path)
		return nil // not err -> continue with next file
	}

	thumbnail, err := checkForThumbnail(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error checking for thumbnail of file=%s, error: %s", path, err)
		return nil // not err -> continue with next file
	}

	artist, title, panel, err := parseExifUserComment(path, usercomment)
	updateArtworkRecord(path, artist, title, panel, thumbnail)
	return nil
}

func checkForThumbnail(path string) (string, error) {
	var re = regexp.MustCompile(`^(.*)(\.(jpg|jpeg|JPG|JPEG))`)
	s := re.ReplaceAllString(path, `${1}_tn${2}`)

	_, err := os.Stat(s)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			/* no thumbnail for this image */
			return "", nil
		} else {
			return "", err
		}
	}
	return s, nil
}

func parseExifUserComment(path string, usercomment string) (string, string, string, error) {
	lines := strings.Split(usercomment, "\n")

	if len(lines) < 4 {
		fmt.Fprintf(os.Stderr, "warn: short exif data, path=%s\n", path)
		return "", "", "", errors.New("short exif data, path: " + path)
	}

	/* verifz format version */
	tag_version, err := splitLine(path, lines[0], "ef-artshow-tags-version")
	if err != nil {
		return "", "", "", err
	}
	if tag_version != "v1" {
		fmt.Fprintf(os.Stderr, "warn: unexpected tags version: %s, path=%s\n", tag_version, path)
		return "", "", "", errors.New("unexpected tags version: " + tag_version + ", file=" + path)
	}

	artist, err := splitLine(path, lines[1], "artist")
	if err != nil {
		return "", "", "", err
	}
	title, err := splitLine(path, lines[2], "title")
	if err != nil {
		return "", "", "", err
	}
	panel, err := splitLine(path, lines[3], "panel")
	if err != nil {
		return "", "", "", err
	}

	return artist, title, panel, nil
}

func splitLine(path string, line string, expected_key string) (value string, err error) {
	key, value, found := strings.Cut(line, ":")
	if !found {
		fmt.Fprintf(os.Stderr, "warn: parse error, no colon found in exif line, path=%s, %s\n", path)
		return "", fmt.Errorf("splitLine: no colon found")
	}
	key = strings.Trim(key, " \n")
	value = strings.Trim(value, " \n")
	if key != expected_key {
		fmt.Fprintf(os.Stderr, "warn: parse error, unexpected key: '%s', expected '%s',, path=%s, %s\n", key, expected_key, path, err)
		return "", fmt.Errorf("unexpected key")
	}
	return value, nil
}

func updateArtworkRecord(path string, artist string, title string, panel string, thumbnail string) {
	path = path[len(config.ImagePath()):]
	if thumbnail != "" {
		thumbnail = thumbnail[len(config.ImagePath()):]
	}
	db, err := database.GetDB();
	a, err := db.GetArtworkByFilename(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching db record for file '%s'. %s\n", path, err)
		return
	}
	if a != nil {
		// file is already know to our database -> do nothing
		if a.Title == title && a.Artist == artist && a.Panel == panel && a.Thumbnail == thumbnail {
			return
		}
		a.Title = title
		a.Artist = artist
		a.Panel = panel
		a.Thumbnail = thumbnail
		db.UpdateArtwork(a)
		return
	}
	a = &database.Artwork{
		Title: title,
		Artist: artist,
		Panel: panel,
		Filename: path,
		Thumbnail: thumbnail,
		EloRating: int16(config.RatingDefaultPoints()),
		DuelCount: 0,
	}
	err = db.AddArtwork(a)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating db record for file '%s'. %s\n", path, err)
	}
	return
}
