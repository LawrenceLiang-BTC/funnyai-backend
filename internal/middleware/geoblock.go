package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// 中国大陆IP段（简化版，实际生产环境建议使用IP数据库如MaxMind）
// 这里只是示例，实际应该使用更完整的IP库
var chinaIPRanges = []string{
	"1.0.0.0/8",
	"14.0.0.0/8",
	"27.0.0.0/8",
	"36.0.0.0/8",
	"39.0.0.0/8",
	"42.0.0.0/8",
	"49.0.0.0/8",
	"58.0.0.0/8",
	"59.0.0.0/8",
	"60.0.0.0/8",
	"61.0.0.0/8",
	"101.0.0.0/8",
	"103.0.0.0/8",
	"106.0.0.0/8",
	"110.0.0.0/8",
	"111.0.0.0/8",
	"112.0.0.0/8",
	"113.0.0.0/8",
	"114.0.0.0/8",
	"115.0.0.0/8",
	"116.0.0.0/8",
	"117.0.0.0/8",
	"118.0.0.0/8",
	"119.0.0.0/8",
	"120.0.0.0/8",
	"121.0.0.0/8",
	"122.0.0.0/8",
	"123.0.0.0/8",
	"124.0.0.0/8",
	"125.0.0.0/8",
	"126.0.0.0/8",
	"139.0.0.0/8",
	"140.0.0.0/8",
	"144.0.0.0/8",
	"150.0.0.0/8",
	"153.0.0.0/8",
	"157.0.0.0/8",
	"163.0.0.0/8",
	"171.0.0.0/8",
	"175.0.0.0/8",
	"180.0.0.0/8",
	"182.0.0.0/8",
	"183.0.0.0/8",
	"202.0.0.0/8",
	"203.0.0.0/8",
	"210.0.0.0/8",
	"211.0.0.0/8",
	"218.0.0.0/8",
	"219.0.0.0/8",
	"220.0.0.0/8",
	"221.0.0.0/8",
	"222.0.0.0/8",
	"223.0.0.0/8",
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
		// X-Forwarded-For 可能包含多个IP，第一个是原始客户端IP
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

	// 检查是否在中国IP段内
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
