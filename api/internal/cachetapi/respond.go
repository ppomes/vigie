package cachetapi

import (
	"crypto/rand"
	"encoding/hex"
	"math"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// item answers with Cachet's single-object envelope.
func item(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// fail answers with Cachet's JSON:API error envelope.
func fail(c *gin.Context, status int, detail string) {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	c.AbortWithStatusJSON(status, gin.H{"errors": []gin.H{{
		"id":     hex.EncodeToString(b),
		"status": strconv.Itoa(status),
		"title":  http.StatusText(status),
		"detail": detail,
	}}})
}

// page reads Cachet's per_page (default 20) and page parameters.
func page(c *gin.Context) (perPage, current int) {
	perPage, _ = strconv.Atoi(c.DefaultQuery("per_page", "20"))
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 1000 {
		perPage = 1000
	}
	current, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	if current < 1 {
		current = 1
	}
	return perPage, current
}

// paginate counts q, loads one page into dest, and returns Cachet's
// pagination metadata.
func paginate(c *gin.Context, q *gorm.DB, dest any) (gin.H, error) {
	perPage, current := page(c)
	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, err
	}
	if err := q.Session(&gorm.Session{}).Limit(perPage).Offset((current - 1) * perPage).Find(dest).Error; err != nil {
		return nil, err
	}
	pages := int(math.Ceil(float64(total) / float64(perPage)))
	if pages < 1 {
		pages = 1
	}
	link := func(p int) any {
		if p < 1 || p > pages {
			return nil
		}
		u := *c.Request.URL
		v := u.Query()
		v.Set("page", strconv.Itoa(p))
		u.RawQuery = v.Encode()
		return (&url.URL{Path: u.Path, RawQuery: u.RawQuery}).String()
	}
	count := int(total) - (current-1)*perPage
	if count > perPage {
		count = perPage
	}
	if count < 0 {
		count = 0
	}
	return gin.H{"pagination": gin.H{
		"total":        total,
		"count":        count,
		"per_page":     perPage,
		"current_page": current,
		"total_pages":  pages,
		"links": gin.H{
			"next_page":     link(current + 1),
			"previous_page": link(current - 1),
		},
	}}, nil
}

// list answers with a page of items and its metadata.
func list(c *gin.Context, meta gin.H, data any) {
	c.JSON(http.StatusOK, gin.H{"meta": meta, "data": data})
}

// sortQuery applies Cachet's sort and order parameters, limited to the
// given columns (Cachet name to Vigie column). Columns are quoted for each
// database, since "order" is a reserved word.
func sortQuery(c *gin.Context, q *gorm.DB, allowed map[string]string, fallback ...clause.OrderByColumn) *gorm.DB {
	col, ok := allowed[c.Query("sort")]
	if !ok {
		for _, o := range fallback {
			q = q.Order(o)
		}
		return q
	}
	return q.Order(clause.OrderByColumn{Column: clause.Column{Name: col}, Desc: c.Query("order") == "desc"}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "id"}})
}

func by(col string, desc bool) clause.OrderByColumn {
	return clause.OrderByColumn{Column: clause.Column{Name: col}, Desc: desc}
}
