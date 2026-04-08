package handlers

// OpenAPI 3.0 规范，描述 API v1 的所有端点
const openAPISpec = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Skills Hub API",
    "description": "OpenClaw 技能市场 API，提供技能搜索、详情、下载、上传、分类和统计等功能。所有请求需要通过 API Key 认证（Bearer Token）。",
    "version": "1.0.0",
    "contact": { "name": "Skills Hub", "url": "https://skills-hub.example.com" }
  },
  "servers": [
    { "url": "/api/v1", "description": "API v1" }
  ],
  "security": [{ "bearerAuth": [] }],
  "components": {
    "securitySchemes": {
      "bearerAuth": {
        "type": "http",
        "scheme": "bearer",
        "description": "在个人中心生成的 API Key，格式: shk_xxxx"
      }
    },
    "schemas": {
      "Skill": {
        "type": "object",
        "properties": {
          "id": { "type": "integer", "description": "技能 ID" },
          "slug": { "type": "string", "description": "技能标识符" },
          "name": { "type": "string", "description": "技能名称" },
          "description": { "type": "string", "description": "技能描述" },
          "version": { "type": "string", "description": "版本号" },
          "author": { "type": "string", "description": "作者" },
          "rating": { "type": "number", "format": "float", "description": "用户评分均值" },
          "download_count": { "type": "integer", "description": "下载次数" },
          "category": { "type": "string", "description": "分类（逗号分隔）" }
        }
      },
      "SkillDetail": {
        "allOf": [
          { "$ref": "#/components/schemas/Skill" },
          {
            "type": "object",
            "properties": {
              "readme": { "type": "string", "description": "SKILL.md 渲染后的 HTML" },
              "rating_count": { "type": "integer", "description": "评分人数" },
              "keywords": { "type": "array", "items": { "type": "string" }, "description": "关键词列表" }
            }
          }
        ]
      },
      "Category": {
        "type": "object",
        "properties": {
          "name": { "type": "string" },
          "count": { "type": "integer" }
        }
      },
      "Stats": {
        "type": "object",
        "properties": {
          "total_skills": { "type": "integer" },
          "total_users": { "type": "integer" },
          "total_tenants": { "type": "integer" },
          "total_comments": { "type": "integer" },
          "total_ratings": { "type": "integer" }
        }
      },
      "Error": {
        "type": "object",
        "properties": {
          "error": { "type": "string", "description": "错误描述" }
        }
      }
    }
  },
  "paths": {
    "/search": {
      "get": {
        "summary": "搜索技能",
        "description": "按关键词、分类搜索技能列表，支持分页和排序。",
        "parameters": [
          { "name": "q", "in": "query", "schema": { "type": "string" }, "description": "搜索关键词" },
          { "name": "category", "in": "query", "schema": { "type": "string" }, "description": "分类筛选" },
          { "name": "sort", "in": "query", "schema": { "type": "string", "enum": ["rating", "newest"] }, "description": "排序方式" },
          { "name": "page", "in": "query", "schema": { "type": "integer", "default": 1 }, "description": "页码" },
          { "name": "per_page", "in": "query", "schema": { "type": "integer", "default": 20, "maximum": 100 }, "description": "每页数量" },
          { "name": "tenant_id", "in": "query", "schema": { "type": "integer" }, "description": "租户 ID（可选，限定范围）" }
        ],
        "responses": {
          "200": {
            "description": "搜索结果",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "skills": { "type": "array", "items": { "$ref": "#/components/schemas/Skill" } },
                    "page": { "type": "integer" },
                    "per_page": { "type": "integer" },
                    "total": { "type": "integer" }
                  }
                }
              }
            }
          },
          "401": { "description": "未认证", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Error" } } } }
        }
      }
    },
    "/skills/{slug}": {
      "get": {
        "summary": "技能详情",
        "description": "获取单个技能的完整信息，包括渲染后的 README 和关键词。",
        "parameters": [
          { "name": "slug", "in": "path", "required": true, "schema": { "type": "string" }, "description": "技能标识符" },
          { "name": "tenant_id", "in": "query", "schema": { "type": "integer" }, "description": "租户 ID" }
        ],
        "responses": {
          "200": { "description": "技能详情", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/SkillDetail" } } } },
          "404": { "description": "技能不存在" }
        }
      }
    },
    "/download/{id}": {
      "get": {
        "summary": "下载技能 ZIP",
        "description": "下载技能的 ZIP 包。上传的技能返回原始 ZIP，同步的技能按 SKILL.md 打包。",
        "parameters": [
          { "name": "id", "in": "path", "required": true, "schema": { "type": "integer" }, "description": "技能 ID" }
        ],
        "responses": {
          "200": { "description": "ZIP 文件", "content": { "application/zip": {} } },
          "404": { "description": "技能不存在" }
        }
      }
    },
    "/upload": {
      "post": {
        "summary": "上传技能",
        "description": "上传包含 SKILL.md 的 ZIP 文件，提交审核。",
        "requestBody": {
          "required": true,
          "content": {
            "multipart/form-data": {
              "schema": {
                "type": "object",
                "properties": {
                  "zipfile": { "type": "string", "format": "binary", "description": "ZIP 文件（最大 10MB）" },
                  "tenant_id": { "type": "integer", "description": "目标租户 ID（可选）" }
                },
                "required": ["zipfile"]
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": "上传成功",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "id": { "type": "integer" },
                    "slug": { "type": "string" },
                    "review_status": { "type": "string" },
                    "message": { "type": "string" }
                  }
                }
              }
            }
          }
        }
      }
    },
    "/categories": {
      "get": {
        "summary": "分类列表",
        "description": "获取所有技能分类及其计数，按数量降序排列。",
        "parameters": [
          { "name": "tenant_id", "in": "query", "schema": { "type": "integer" }, "description": "租户 ID" }
        ],
        "responses": {
          "200": {
            "description": "分类列表",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "categories": { "type": "array", "items": { "$ref": "#/components/schemas/Category" } }
                  }
                }
              }
            }
          }
        }
      }
    },
    "/stats": {
      "get": {
        "summary": "平台统计",
        "description": "获取平台整体统计数据。",
        "responses": {
          "200": { "description": "统计数据", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Stats" } } } }
        }
      }
    }
  }
}`
