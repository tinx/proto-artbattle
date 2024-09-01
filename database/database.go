package database

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/driver/mysql"
)

type Artwork struct {
	gorm.Model
	Title		string	`gorm:"type:varchar(120); NOT NULL"`
	Artist		string	`gorm:"type:varchar(120); NOT NULL"`
	Panel		string	`gorm:"type:varchar(10); NOT NULL"`
	Filename	string	`gorm:"type:varchar(120); NOT NULL"`
	DuelCount	uint64	`gorm:"index:idx_duel_count"`
	EloRating	uint16	`gorm:"index:idx_elo_rating"`
}

type MysqlRepository struct {
	db	*gorm.DB
}

var _db *MysqlRepository

func GetDB() (*MysqlRepository, error) {
	return _db, nil
}

func Create() *MysqlRepository {
	_db = &MysqlRepository{}
	return _db
}

func (r *MysqlRepository) Open(dsn string) error {
	gormConfig := &gorm.Config{}
	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return err
	}
	r.db = db
	return nil
}

func (r *MysqlRepository) Close() {
	_db = nil
	// no-op in Gorm v2
}

func (r *MysqlRepository) Migrate() error {
	err := r.db.AutoMigrate(
		&Artwork{},
	)
	if err != nil {
		return err
	}
	return nil
}

func (r *MysqlRepository) AddArtwork(a *Artwork) error {
	err := r.db.Create(a).Error
	if err != nil {
		return err
	}
	return nil
}

func (r *MysqlRepository) RemoveArtwork(a *Artwork) error {
	err := r.db.Delete(a).Error
	if err != nil {
		return err
	}
	return nil
}

func (r *MysqlRepository) GetArtworkById(id int64) (*Artwork, error) {
	var a Artwork
	err := r.db.First(&a, id).Error
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *MysqlRepository) GetArtworkByFilename(filename string) (*Artwork, error) {
	var a Artwork
	result := r.db.Where("filename = ?", filename).First(&a)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &a, nil
}

func (r *MysqlRepository) GetArtworkWithLowestDuelCount() (*Artwork, error) {
	var a Artwork
	err := r.db.Order("duel_count asc").Limit(1).First(&a).Error
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *MysqlRepository) GetArtworksWithSimilarEloRating(benchmark *Artwork, count int) ([]*Artwork, error) {
	/* step 1: load up to 'count' artworks with elo higher than benchmark
	   step 2: load up to 'count - row_count' artworks with lower or
	   	   equal elo. Push out higher ones with lower ones */
	var res []*Artwork
	rows, err := r.db.Model(&Artwork{}).Where("elo_rating > ?", benchmark.EloRating).Order("elo_rating asc").Limit(count).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lc int = 0
	for rows.Next() {
		var a Artwork
		r.db.ScanRows(rows, &a)
		res = append(res, &a)
		lc = lc + 1
	}
	/* how many more rows do we want? */
	var remaining_count int
	if lc < (count / 2) {
		remaining_count = count - lc
	} else {
		remaining_count = (count / 2)
	}
	rows2, err := r.db.Model(&Artwork{}).Where("elo_rating <= ? and id != ?", benchmark.EloRating, benchmark.ID).Order("elo_rating desc").Limit(remaining_count).Rows()
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var a Artwork
		r.db.ScanRows(rows2, &a)
		lc = lc + 1
		res = append([]*Artwork{&a}, res...)
	}
	if len(res) < count {
		count = len(res)
	}
	return res[:count], nil
}

func (r *MysqlRepository) UpdateArtwork(a *Artwork) error {
	err := r.db.Save(a).Error
	if err != nil {
		return err
	}
	return nil
}

func (r *MysqlRepository) GetArtworkRank(a *Artwork) (int64, error) {
	var count int64
 	r.db.Model(&Artwork{}).Where("elo_rating > ?", a.EloRating).Count(&count)
	return count + 1, nil
}
