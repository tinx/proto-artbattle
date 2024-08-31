package imagescan

import (
	"path/filepath"
	"fmt"
	"io/ioutil"
	"os"

//	"github.com/rwcarlsen/goexif/exif"
	"github.com/dsoprea/go-exif"
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

	fmt.Printf("%s\n", path)
	if info.IsDir() {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "file walk error: %s\n", err)
		return err
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "file read error: %s\n", err)
		return err
	}

	fmt.Printf("read %d bytes\n", len(data))

	rawExif, err := exif.SearchAndExtractExif(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "find exif error: %s\n", err)
		return err
	}

	im := exif.NewIfdMappingWithStandard()
	ti := exif.NewTagIndex()

	visitor := func(fqIfdPath string, ifdIndex int, tagId uint16, tagType exif.TagType, valueContext exif.ValueContext) (error) {
		ifdPath, err := im.StripPathPhraseIndices(fqIfdPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "strip path error: %s\n", err)
			return err
		}
		it, err := ti.Get(ifdPath, tagId)
		if err != nil {
			fmt.Fprintf(os.Stderr, "exif get error: %s\n", err)
			return err
		}

		if it.Name != "UserComment" {
			return nil
		}

		valueString := ""
		var value interface{}
		fmt.Println("tag name: " + it.Name)
		if tagType.Type() == exif.TypeUndefined {
			value, err = valueContext.Undefined()
			if err != nil {
				fmt.Fprintf(os.Stderr, "exif value context error: %s\n", err)
				return err
			}
			tmp, ok := value.(exif.TagUnknownType_9298_UserComment)
			if !ok {
				fmt.Fprintf(os.Stderr, "unexpected unknow type error, expected very specific unknown type 9298. error:  %s\n", err)
				return err
			}
			tmp2, _ := tmp.ValueBytes()
			tmp2 = tmp2[5:]
			valueString = string(tmp2)
		} else {
			valueString, err = valueContext.FormatFirst()
		}
		fmt.Println("tag value: " + valueString)

		return nil
	}

	_, err = exif.Visit(exif.IfdStandard, im, ti, rawExif, visitor)
	if err != nil {
		fmt.Fprintf(os.Stderr, "exif visitor error: %s\n", err)
		return err
	}

	/*
	jmp := jpegstructure.NewJpegMediaParser();
	intfc, err := jmp.ParseFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "file walk error: %s\n", err)
		return err
	}

	sl := intfc.(*SegmentList)
	sl.Print()
	*/

	/*
	x,err := exif.Decode(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "exif decode error: %s\n", err)
		return err
	}

	comment, err := x.Get(exif.UserComment)
	fmt.Println(comment)
	*/

	return nil
}
