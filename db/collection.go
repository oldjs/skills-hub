package db

import (
	"time"

	"skills-hub/security"
)

type Collection struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"userId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"isPublic"`
	ItemCount   int       `json:"itemCount"`
	CreatedAt   time.Time `json:"createdAt"`
	// join
	UserName string `json:"userName"`
}

func CreateCollection(userID int64, name, description string, isPublic bool) (int64, error) {
	name = security.EscapePlainText(name)
	description = security.EscapePlainText(description)
	pub := 0
	if isPublic {
		pub = 1
	}
	result, err := GetDB().Exec(`INSERT INTO skill_collections (user_id, name, description, is_public) VALUES (?, ?, ?, ?)`,
		userID, name, description, pub)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func GetCollection(id int64) (*Collection, error) {
	var c Collection
	var pub int
	err := GetDB().QueryRow(`
		SELECT c.id, c.user_id, c.name, c.description, c.is_public, c.created_at,
		       COALESCE(u.display_name, ''), (SELECT COUNT(*) FROM collection_items ci WHERE ci.collection_id = c.id)
		FROM skill_collections c
		JOIN users u ON u.id = c.user_id
		WHERE c.id = ?
	`, id).Scan(&c.ID, &c.UserID, &c.Name, &c.Description, &pub, &c.CreatedAt, &c.UserName, &c.ItemCount)
	if err != nil {
		return nil, err
	}
	c.IsPublic = pub == 1
	c.Name = security.DecodeStoredText(c.Name)
	c.Description = security.DecodeStoredText(c.Description)
	c.UserName = security.DecodeStoredText(c.UserName)
	return &c, nil
}

func ListUserCollections(userID int64) ([]Collection, error) {
	rows, err := GetDB().Query(`
		SELECT c.id, c.user_id, c.name, c.description, c.is_public, c.created_at,
		       (SELECT COUNT(*) FROM collection_items ci WHERE ci.collection_id = c.id)
		FROM skill_collections c WHERE c.user_id = ?
		ORDER BY c.updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Collection
	for rows.Next() {
		var c Collection
		var pub int
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Description, &pub, &c.CreatedAt, &c.ItemCount); err != nil {
			return nil, err
		}
		c.IsPublic = pub == 1
		c.Name = security.DecodeStoredText(c.Name)
		c.Description = security.DecodeStoredText(c.Description)
		list = append(list, c)
	}
	return list, rows.Err()
}

func AddToCollection(collectionID, skillID int64) error {
	_, err := GetDB().Exec(`INSERT OR IGNORE INTO collection_items (collection_id, skill_id) VALUES (?, ?)`, collectionID, skillID)
	return err
}

func RemoveFromCollection(collectionID, skillID int64) error {
	_, err := GetDB().Exec(`DELETE FROM collection_items WHERE collection_id = ? AND skill_id = ?`, collectionID, skillID)
	return err
}

func DeleteCollection(collectionID, userID int64) error {
	_, err := GetDB().Exec(`DELETE FROM skill_collections WHERE id = ? AND user_id = ?`, collectionID, userID)
	return err
}
