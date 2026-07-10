package server

import (
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetAccountTransactions(db *gorm.DB, account string) gin.H {
	postings := query.Init(db).AccountPrefix(account).All()
	return gin.H{"postings": postings}
}
