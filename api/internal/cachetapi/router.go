package cachetapi

import (
	"api/internal/middleware"
	"api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Prefix is where the Cachet compatible API is served. Cachet clients take
// it as their base URL, such as https://status.example.com/cachet/api/v1.
const Prefix = "/cachet/api/v1"

// Register mounts the Cachet compatible API. Reads are open, as in Cachet,
// and show more to authenticated callers. Writes need an API key (or a
// session) of a user allowed to make changes.
func Register(r *gin.Engine, db *gorm.DB, ns *services.NotificationService) {
	read := r.Group(Prefix, middleware.OptionalAuth(db))
	read.GET("/ping", ping)
	read.GET("/version", version)
	read.GET("/status", status(db))

	read.GET("/components", listComponents(db))
	read.GET("/components/groups", listGroups(db))
	read.GET("/components/groups/:id", showGroup(db))
	read.GET("/components/:id", showComponent(db))

	read.GET("/incidents", listIncidents(db))
	read.GET("/incidents/:id", showIncident(db))
	read.GET("/incidents/:id/updates", listUpdates(db))
	read.GET("/incidents/:id/updates/:update_id", showUpdate(db))

	read.GET("/schedules", listSchedules(db))
	read.GET("/schedules/:id", showSchedule(db))

	write := r.Group(Prefix, middleware.AuthRequiredWithDB(db), middleware.RequireAdmin())
	write.POST("/components", createComponent(db))
	write.PUT("/components/:id", updateComponent(db))
	write.DELETE("/components/:id", deleteComponent(db))

	write.POST("/components/groups", createGroup(db))
	write.PUT("/components/groups/:id", updateGroup(db))
	write.DELETE("/components/groups/:id", deleteGroup(db))

	write.POST("/incidents", createIncident(db, ns))
	write.PUT("/incidents/:id", updateIncident(db, ns))
	write.DELETE("/incidents/:id", deleteIncident(db))

	write.POST("/incidents/:id/updates", createUpdate(db, ns))
	write.PUT("/incidents/:id/updates/:update_id", updateUpdate(db))
	write.DELETE("/incidents/:id/updates/:update_id", deleteUpdate(db))

	write.POST("/schedules", createSchedule(db, ns))
	write.PUT("/schedules/:id", updateSchedule(db, ns))
	write.DELETE("/schedules/:id", deleteSchedule(db))
}
