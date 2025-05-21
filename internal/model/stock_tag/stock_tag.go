package stock_tag

import (
	"strings"

	"gorm.io/gorm"
)

type StockTag struct {
	ID    uint   `gorm:"primaryKey" json:"id"`
	Tag   string `gorm:"uniqueIndex;not null;size:255" json:"tag"`
	Color string `json:"color"`
}

// BeforeSave is a GORM hook to normalize the tag before saving
func (s *StockTag) BeforeSave(tx *gorm.DB) error {
	// Convert to lowercase and trim spaces
	s.Tag = strings.TrimSpace(strings.ToLower(s.Tag))
	return nil
}

type StockTagAssociation struct {
	ID     uint     `gorm:"primaryKey" json:"id"`
	Symbol string   `gorm:"index" json:"symbol"`
	TagID  uint     `gorm:"index" json:"tagId"`
	Tag    StockTag `gorm:"foreignKey:TagID" json:"tag"`
}

func (StockTag) TableName() string {
	return "stock_tags"
}

func (StockTagAssociation) TableName() string {
	return "stock_tag_associations"
}

func GetTags(db *gorm.DB, symbol string) ([]StockTag, error) {
	var tags []StockTag
	result := db.Joins("JOIN stock_tag_associations ON stock_tag_associations.tag_id = stock_tags.id").
		Where("stock_tag_associations.symbol = ?", symbol).
		Find(&tags)
	return tags, result.Error
}

func AddTag(db *gorm.DB, symbol string, tag string, color string) error {
	// First create or get the tag
	var stockTag StockTag
	result := db.Where("tag = ?", tag).First(&stockTag)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			stockTag = StockTag{Tag: tag, Color: color}
			if err := db.Create(&stockTag).Error; err != nil {
				return err
			}
		} else {
			return result.Error
		}
	}

	// Then create the association
	return db.Create(&StockTagAssociation{
		Symbol: symbol,
		TagID:  stockTag.ID,
	}).Error
}

func RemoveTag(db *gorm.DB, symbol string, tag string) error {
	// First get the tag ID
	var stockTag StockTag
	if err := db.Where("tag = ?", tag).First(&stockTag).Error; err != nil {
		return err
	}

	// Delete the association
	if err := db.Where("symbol = ? AND tag_id = ?", symbol, stockTag.ID).
		Delete(&StockTagAssociation{}).Error; err != nil {
		return err
	}

	// Check if there are any other associations for this tag
	var count int64
	if err := db.Model(&StockTagAssociation{}).
		Where("tag_id = ?", stockTag.ID).
		Count(&count).Error; err != nil {
		return err
	}

	// If no other associations exist, delete the tag
	if count == 0 {
		if err := db.Delete(&stockTag).Error; err != nil {
			return err
		}
	}

	return nil
}

func GetAllTags(db *gorm.DB) (map[string][]StockTag, error) {
	var associations []StockTagAssociation
	result := db.Preload("Tag").Find(&associations)
	if result.Error != nil {
		return nil, result.Error
	}

	tagMap := make(map[string][]StockTag)
	for _, assoc := range associations {
		tagMap[assoc.Symbol] = append(tagMap[assoc.Symbol], assoc.Tag)
	}
	return tagMap, nil
}
