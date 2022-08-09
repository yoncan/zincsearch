/* Copyright 2022 Zinc Labs Inc. and Contributors
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
 */

package routes

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/zinclabs/zinc/docs" // docs is generated by Swag CLI

	"github.com/zinclabs/zinc"
	"github.com/zinclabs/zinc/pkg/handlers/auth"
	"github.com/zinclabs/zinc/pkg/handlers/cluster"
	"github.com/zinclabs/zinc/pkg/handlers/document"
	"github.com/zinclabs/zinc/pkg/handlers/index"
	"github.com/zinclabs/zinc/pkg/handlers/search"
	"github.com/zinclabs/zinc/pkg/meta"
	"github.com/zinclabs/zinc/pkg/meta/elastic"
)

// SetRoutes sets up all gin HTTP API endpoints that can be called by front end
func SetRoutes(r *gin.Engine) {

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "DELETE", "PUT", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "authorization", "content-type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/", meta.GUI)
	r.GET("/version", meta.GetVersion)
	r.GET("/healthz", meta.GetHealthz)

	// use ginSwagger middleware to serve the API docs
	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	front, err := zinc.GetFrontendAssets()
	if err != nil {
		log.Err(err)
	}

	// UI
	HTTPCacheForUI(r)
	r.StaticFS("/ui/", http.FS(front))
	r.NoRoute(func(c *gin.Context) {
		log.Error().
			Str("method", c.Request.Method).
			Int("code", 404).
			Int("took", 0).
			Msg(c.Request.RequestURI)

		if strings.HasPrefix(c.Request.RequestURI, "/ui/") {
			path := strings.TrimPrefix(c.Request.RequestURI, "/ui/")
			locationPath := strings.Repeat("../", strings.Count(path, "/"))
			c.Status(http.StatusFound)
			c.Writer.Header().Set("Location", "./"+locationPath)
		}
	})

	// auth
	r.POST("/api/login", auth.Login)
	r.POST("/api/user", AuthMiddleware, auth.CreateUpdate)
	r.PUT("/api/user", AuthMiddleware, auth.CreateUpdate)
	r.DELETE("/api/user/:id", AuthMiddleware, auth.Delete)
	r.GET("/api/user", AuthMiddleware, auth.List)

	// cluster
	r.GET("/api/node/status", cluster.NodeStatus)
	r.GET("/api/cluster/status", AuthMiddleware, cluster.ClusterStatus)
	r.GET("/api/shards/distribution", AuthMiddleware, cluster.ShardsDistribution)

	// index
	r.GET("/api/index", AuthMiddleware, index.List)
	r.GET("/api/index_name", AuthMiddleware, index.IndexNameList)
	r.POST("/api/index", AuthMiddleware, index.Create)
	r.PUT("/api/index", AuthMiddleware, index.Create)
	r.PUT("/api/index/:target", AuthMiddleware, index.Create)
	r.GET("/api/index/:target", AuthMiddleware, index.Get)
	r.HEAD("/api/index/:target", AuthMiddleware, index.Exists)
	r.DELETE("/api/index/:target", AuthMiddleware, index.Delete)
	r.POST("/api/index/:target/refresh", AuthMiddleware, index.Refresh)
	// index settings
	r.GET("/api/:target/_mapping", AuthMiddleware, index.GetMapping)
	r.PUT("/api/:target/_mapping", AuthMiddleware, index.SetMapping)
	r.GET("/api/:target/_settings", AuthMiddleware, index.GetSettings)
	r.PUT("/api/:target/_settings", AuthMiddleware, index.SetSettings)
	// analyze
	r.POST("/api/_analyze", AuthMiddleware, index.Analyze)
	r.POST("/api/:target/_analyze", AuthMiddleware, index.Analyze)

	// search
	r.POST("/api/:target/_search", AuthMiddleware, search.SearchV1)

	// document
	// Document Bulk update/insert
	r.POST("/api/_bulk", AuthMiddleware, document.Bulk)
	r.POST("/api/:target/_bulk", AuthMiddleware, document.Bulk)
	r.POST("/api/:target/_multi", AuthMiddleware, document.Multi)
	r.POST("/api/_bulkv2", AuthMiddleware, document.Bulkv2)         // New JSON format
	r.POST("/api/:target/_bulkv2", AuthMiddleware, document.Bulkv2) // New JSON format
	// Document CRUD APIs. Update is same as create.
	r.POST("/api/:target/_doc", AuthMiddleware, document.CreateUpdate)    // create
	r.PUT("/api/:target/_doc", AuthMiddleware, document.CreateUpdate)     // create
	r.PUT("/api/:target/_doc/:id", AuthMiddleware, document.CreateUpdate) // create or update
	r.POST("/api/:target/_update/:id", AuthMiddleware, document.Update)   // update
	r.DELETE("/api/:target/_doc/:id", AuthMiddleware, document.Delete)    // delete

	/**
	 * elastic compatible APIs
	 */

	r.GET("/es/", ESMiddleware, func(c *gin.Context) {
		c.JSON(http.StatusOK, elastic.NewESInfo(c))
	})
	r.HEAD("/es/", ESMiddleware, func(c *gin.Context) {
		c.JSON(http.StatusOK, elastic.NewESInfo(c))
	})
	r.GET("/es/_license", ESMiddleware, func(c *gin.Context) {
		c.JSON(http.StatusOK, elastic.NewESLicense(c))
	})
	r.GET("/es/_xpack", ESMiddleware, func(c *gin.Context) {
		c.JSON(http.StatusOK, elastic.NewESXPack(c))
	})

	r.POST("/es/_search", AuthMiddleware, ESMiddleware, search.SearchDSL)
	r.POST("/es/_msearch", AuthMiddleware, ESMiddleware, search.MultipleSearch)
	r.POST("/es/:target/_search", AuthMiddleware, ESMiddleware, search.SearchDSL)
	r.POST("/es/:target/_msearch", AuthMiddleware, ESMiddleware, search.MultipleSearch)

	r.GET("/es/_index_template", AuthMiddleware, ESMiddleware, index.ListTemplate)
	r.POST("/es/_index_template", AuthMiddleware, ESMiddleware, index.CreateTemplate)
	r.PUT("/es/_index_template/:target", AuthMiddleware, ESMiddleware, index.CreateTemplate)
	r.GET("/es/_index_template/:target", AuthMiddleware, ESMiddleware, index.GetTemplate)
	r.HEAD("/es/_index_template/:target", AuthMiddleware, ESMiddleware, index.GetTemplate)
	r.DELETE("/es/_index_template/:target", AuthMiddleware, ESMiddleware, index.DeleteTemplate)
	// ES Compatible data stream
	r.PUT("/es/_data_stream/:target", AuthMiddleware, ESMiddleware, elastic.PutDataStream)
	r.GET("/es/_data_stream/:target", AuthMiddleware, ESMiddleware, elastic.GetDataStream)
	r.HEAD("/es/_data_stream/:target", AuthMiddleware, ESMiddleware, elastic.GetDataStream)

	r.PUT("/es/:target", AuthMiddleware, ESMiddleware, index.CreateES)
	r.HEAD("/es/:target", AuthMiddleware, ESMiddleware, index.Exists)

	r.GET("/es/:target/_mapping", AuthMiddleware, ESMiddleware, index.GetESMapping)
	r.PUT("/es/:target/_mapping", AuthMiddleware, ESMiddleware, index.SetMapping)

	r.GET("/es/:target/_settings", AuthMiddleware, ESMiddleware, index.GetSettings)
	r.PUT("/es/:target/_settings", AuthMiddleware, ESMiddleware, index.SetSettings)

	r.POST("/es/_analyze", AuthMiddleware, ESMiddleware, index.Analyze)
	r.POST("/es/:target/_analyze", AuthMiddleware, ESMiddleware, index.Analyze)

	// ES Bulk update/insert
	r.POST("/es/_bulk", AuthMiddleware, ESMiddleware, document.ESBulk)
	r.POST("/es/:target/_bulk", AuthMiddleware, ESMiddleware, document.ESBulk)
	// ES Document
	r.POST("/es/:target/_doc", AuthMiddleware, ESMiddleware, document.CreateUpdate)        // create
	r.PUT("/es/:target/_doc/:id", AuthMiddleware, ESMiddleware, document.CreateUpdate)     // create or update
	r.PUT("/es/:target/_create/:id", AuthMiddleware, ESMiddleware, document.CreateUpdate)  // create
	r.POST("/es/:target/_create/:id", AuthMiddleware, ESMiddleware, document.CreateUpdate) // create
	r.POST("/es/:target/_update/:id", AuthMiddleware, ESMiddleware, document.Update)       // update part of document
	r.DELETE("/es/:target/_doc/:id", AuthMiddleware, ESMiddleware, document.Delete)        // delete
}
