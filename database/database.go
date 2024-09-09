package database

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/driver/mysql"
)

type Artwork struct {
	gorm.Model
	Title		string	`gorm:"type:varchar(120); NOT NULL"`
	Artist		string	`gorm:"type:varchar(120); NOT NULL"`
	Panel		string	`gorm:"type:varchar(10); NOT NULL"`
	Filename	string	`gorm:"type:varchar(120); NOT NULL"`
	Thumbnail	string	`gorm:"type:varchar(120); NOT NULL"`
	DuelCount	uint64	`gorm:"index:idx_duel_count"`
	EloRating	int16	`gorm:"index:idx_elo_rating"`
}

type Duel struct {
	gorm.Model
	Duelist1	uint		`gorm:"type:bigint; NOT NULL"`
	Duelist2	uint		`gorm:"type:bigint; NOT NULL"`
	Winner		uint		`gorm:"type:bigint; NOT NULL"`
	When		time.Time	`gorm:"NOT NULL"`
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
		&Duel{},
	)
	if err != nil {
		return err
	}
	return nil
}

func (r *MysqlRepository) Transaction(tx func (*gorm.DB) error) error {
	return r.db.Transaction(tx)
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

func (r *MysqlRepository) GetLeaderboard(maxcount int) ([]*Artwork, error) {
	var lb []*Artwork
	rows, err := r.db.Table("artworks").Order("elo_rating desc, id asc").Limit(maxcount).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var a Artwork
		r.db.ScanRows(rows, &a)
		lb = append(lb, &a)
	}
	return lb, nil
}

func (r *MysqlRepository) GetArtworksWithSimilarEloRating(benchmark *Artwork, count int) ([]*Artwork, error) {
	/* step 1: load up to 'count' artworks with elo higher than benchmark
	   step 2: load an approriate number of artworks with lower or
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

func UpdateArtwork(db *gorm.DB, a *Artwork) error {
	err := db.Save(a).Error
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

func GetArtworkRank(db *gorm.DB, a *Artwork) (int64, error) {
	var count int64
	db.Model(&Artwork{}).Where("elo_rating > ?", a.EloRating).Count(&count)
	return count + 1, nil
}

func (r *MysqlRepository) GetTotalDuelCount() (int64, error) {
	var count int64
	r.db.Table("artworks").Select("sum(duel_count)").Row().Scan(&count)
	/* the total number of duels is half the sum of all duel_counts because a duel
	   has two participants. (hence the name) */
	return count / 2, nil
}

func AddDuel(db *gorm.DB, d *Duel) error {
	err := db.Create(d).Error
	if err != nil {
		return err
	}
	return nil
}

