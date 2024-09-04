package imagescan

import (
	"path/filepath"
	"fmt"
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/tinx/proto-artbattle/database"
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

	parseExifUserComment(path, usercomment)
	return nil
}

func parseExifUserComment(path string, usercomment string) {
	lines := strings.Split(usercomment, "\n")

	if len(lines) < 4 {
		fmt.Fprintf(os.Stderr, "warn: short exif data, path=%s\n", path)
		return
	}

	/* verifz format version */
	tag_version, err := splitLine(path, lines[0], "ef-artshow-tags-version")
	if err != nil {
		return
	}
	if tag_version != "v1" {
		fmt.Fprintf(os.Stderr, "warn: unexpected tags version: %s, path=%s\n", tag_version, path)
		return
	}

	artist, err := splitLine(path, lines[1], "artist")
	if err != nil {
		return
	}
	title, err := splitLine(path, lines[2], "title")
	if err != nil {
		return
	}
	panel, err := splitLine(path, lines[3], "panel")
	if err != nil {
		return
	}

	updateArtworkRecord(path, artist, title, panel)
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

func updateArtworkRecord(path string, artist string, title string, panel string) {
	db, err := database.GetDB();
	a, err := db.GetArtworkByFilename(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching db record for file '%s'. %s\n", path, err)
		return
	}
	if a != nil {
		// file is already know to our database -> do nothing
		return
	}
	fmt.Println("No record found, adding to database.")
	a = &database.Artwork{
		Title: title,
		Artist: artist,
		Panel: panel,
		Filename: path,
		EloRating: 800,
		DuelCount: 0,
	}
	err = db.AddArtwork(a)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating db record for file '%s'. %s\n", path, err)
	}
	return
}
