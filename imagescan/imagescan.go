package imagescan

import (
	"path/filepath"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/dsoprea/go-exif"
	"github.com/tinx/proto-artbattle/database"
)

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

	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't open file '%s' error: %s\n", path, err)
		return nil // not err -> continue with next file
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "file read error: %s\n", err)
		return nil // not err -> continue with next file
	}

	rawExif, err := exif.SearchAndExtractExif(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "find exif error: %s\n", err)
		return nil // not err -> continue with next file
	}

	im := exif.NewIfdMappingWithStandard()
	ti := exif.NewTagIndex()

	visitor := func(fqIfdPath string, ifdIndex int, tagId uint16, tagType exif.TagType, valueContext exif.ValueContext) (error) {
		ifdPath, err := im.StripPathPhraseIndices(fqIfdPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "strip path error: %s\n", err)
			return nil // not err -> continue with next file
		}
		it, err := ti.Get(ifdPath, tagId)
		if err != nil {
			fmt.Fprintf(os.Stderr, "exif get error: %s\n", err)
			return nil // not err -> continue with next file
		}

		if it.Name != "UserComment" {
			return nil
		}

		valueString := ""
		var value interface{}
		if tagType.Type() == exif.TypeUndefined {
			value, err = valueContext.Undefined()
			if err != nil {
				fmt.Fprintf(os.Stderr, "exif value context error: %s\n", err)
				return nil // not err -> continue with next file
			}
			tmp, ok := value.(exif.TagUnknownType_9298_UserComment)
			if !ok {
				fmt.Fprintf(os.Stderr, "unexpected unknow type error, expected very specific unknown type 9298. error:  %s\n", err)
				return nil // not err -> continue with next file
			}
			tmp2, _ := tmp.ValueBytes()
			header := tmp2[:5]
			if string(header) != "ASCII" {
				fmt.Fprintf(os.Stderr, "unexpected header\n")
				return nil // not err -> continue with next file
			}
			tmp2 = tmp2[8:] // cut off ASCII plus three null bytes
			valueString = string(tmp2)
		} else {
			valueString, err = valueContext.FormatFirst()
		}

		parseExifUserComment(path, valueString)

		return nil
	}

	_, err = exif.Visit(exif.IfdStandard, im, ti, rawExif, visitor)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: exif visitor error: %s\n", err)
		return nil // not err -> continue with next file
	}

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

	fmt.Printf("tag version: %s\n", tag_version)
	fmt.Printf("TAG artist: %s\n", artist)
	fmt.Printf("TAG title: %s\n", title)
	fmt.Printf("TAG panel: %s\n", panel)
	fmt.Printf("Comment: %s\n", usercomment)

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
