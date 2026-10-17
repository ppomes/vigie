package cachetapi

import (
	"net/http"

	"api/internal/handlers"
	"api/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// cachetVersion is the Cachet version this layer reports, so clients that
// check it accept the server.
const cachetVersion = "2.4.1"

func authenticated(c *gin.Context) bool {
	_, ok := c.Get("user")
	return ok
}

func ping(c *gin.Context) { item(c, "Pong!") }

func version(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"meta": gin.H{
			"on_latest": true,
			"latest":    gin.H{"tag_name": "v" + cachetVersion, "prerelease": false, "draft": false},
		},
		"data": cachetVersion,
	})
}

// status reports the overall status as Cachet does: success, info (some
// issues) or danger (a major outage).
func status(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var components []models.Component
		if err := db.Scopes(handlers.PublicComponents).Find(&components).Error; err != nil {
			fail(c, http.StatusInternalServerError, "Failed to fetch components")
			return
		}
		worst := 1
		for _, comp := range components {
			if s := cachetComponentStatus(comp.Status); s > worst {
				worst = s
			}
		}
		switch {
		case worst >= 4:
			item(c, gin.H{"status": "danger", "message": "Some systems are experiencing major outages"})
		case worst > 1:
			item(c, gin.H{"status": "info", "message": "Some systems are experiencing issues"})
		default:
			item(c, gin.H{"status": "success", "message": "All systems are operational"})
		}
	}
}
