package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// 中国大陆IP段（更精确的版本，排除香港/澳门/台湾）
// 生产环境建议使用MaxMind GeoIP数据库
var chinaIPRanges = []string{
	// 中国电信
	"1.80.0.0/13",
	"1.192.0.0/13",
	"14.16.0.0/12",
	"27.8.0.0/13",
	"36.32.0.0/14",
	"42.48.0.0/15",
	"58.16.0.0/15",
	"59.32.0.0/13",
	"60.0.0.0/13",
	"61.128.0.0/10",
	// 中国联通
	"101.16.0.0/12",
	"106.0.0.0/13",
	"110.80.0.0/13",
	"111.0.0.0/10",
	"112.0.0.0/10",
	"113.0.0.0/10",
	"116.0.0.0/12",
	"117.32.0.0/13",
	"118.72.0.0/13",
	"119.0.0.0/13",
	"120.0.0.0/12",
	"121.0.0.0/12",
	"122.0.0.0/12",
	"123.0.0.0/11",
	"124.64.0.0/12",
	"125.32.0.0/12",
	// 中国移动
	"39.128.0.0/10",
	"111.0.0.0/10",
	"112.0.0.0/10",
	"117.128.0.0/10",
	"120.192.0.0/10",
	"183.192.0.0/10",
	"211.136.0.0/13",
	"218.200.0.0/13",
	"221.176.0.0/13",
	"223.64.0.0/11",
}

var chinaIPNets []*net.IPNet

func init() {
	for _, cidr := range chinaIPRanges {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			chinaIPNets = append(chinaIPNets, ipNet)
		}
	}
}

// GeoBlock 地理位置限制中间件
func GeoBlock(enabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled {
			c.Next()
			return
		}

		clientIP := getClientIP(c)
		
		if isBlockedIP(clientIP) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "Service not available in your region",
				"code":    "GEO_BLOCKED",
				"message": "This service is not available in your country/region due to regulatory requirements.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// getClientIP 获取客户端真实IP
func getClientIP(c *gin.Context) string {
	// 检查 X-Forwarded-For 头（用于代理/CDN）
	xff := c.GetHeader("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// 检查 X-Real-IP 头
	xri := c.GetHeader("X-Real-IP")
	if xri != "" {
		return xri
	}

	// 检查 CF-Connecting-IP (Cloudflare)
	cfip := c.GetHeader("CF-Connecting-IP")
	if cfip != "" {
		return cfip
	}

	// 回退到 RemoteAddr
	ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return ip
}

// isBlockedIP 检查IP是否被封锁
func isBlockedIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	for _, ipNet := range chinaIPNets {
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}

// GeoBlockWithConfig 带配置的地理限制中间件
func GeoBlockWithConfig(enabled bool, blockedCountries []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled {
			c.Next()
			return
		}

		// 检查是否有国家代码头（通常由CDN如Cloudflare添加）
		countryCode := c.GetHeader("CF-IPCountry")
		if countryCode != "" {
			for _, blocked := range blockedCountries {
				if strings.EqualFold(countryCode, blocked) {
					c.JSON(http.StatusForbidden, gin.H{
						"error":   "Service not available in your region",
						"code":    "GEO_BLOCKED",
						"message": "This service is not available in your country/region due to regulatory requirements.",
					})
					c.Abort()
					return
				}
			}
			// 如果有CF头且不在黑名单，直接放行
			c.Next()
			return
		}

		// 回退到IP检查
		clientIP := getClientIP(c)
		if isBlockedIP(clientIP) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "Service not available in your region",
				"code":    "GEO_BLOCKED",
				"message": "This service is not available in your country/region due to regulatory requirements.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
