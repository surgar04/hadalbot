# 第三方来源及许可声明

根目录LICENSE适用于HadalBot原创代码和项目自行编写的文档。第三方组件、资料和品牌保留原有权利及适用条款，本项目不额外授予这些内容的权利。

## Go运行时与模块依赖

以下清单依据本次发布准备时 `go list -deps` 对Windows/Linux amd64的cmd/hadalbot和cmd/migrate结果合并整理，版本以go.mod/go.sum为准。许可和已有NOTICE/COPYRIGHT声明从对应模块原文复制到licenses，未修改许可正文。Go标准库及运行时许可另见 [Go-LICENSE](licenses/Go-LICENSE)。

开发工具、TypeScript编译器、Node、数据库、Redis及容器基础镜像没有作为运行时二进制随本仓库分发；使用或重新分发它们时遵循各自的许可。后续依赖升级或打包范围变化时需同步核对清单和随附声明。

## 上游接口资料

| 内容 | 来源与范围 |
|---|---|
| 千寻HTTP/事件规范及示例 | [千寻官方接口文档](https://qxpro1.apifox.cn/api-346683073)、[事件文档](https://qxpro1.apifox.cn/doc-4379573)。docs/contracts/qianxun/official.json保留公开接口页面快照及来源，pkg/qianxun/contracts.json含派生的协议描述和事件正文。 |
| 米哈游及相关上游契约 | docs/contracts/mihoyo及pkg/mihoyo/contracts.json，结合项目既有规范和请求实现整理；每项保留来源、端点和候选标记。字段描述及上游示例按各自来源权利处理。 |
| 原版example测试夹具 | pkg/plugin/process/testdata/legacy，保留本项目旧版的小型示例以验证兼容性；没有复制整套旧业务运行时。 |

上游文档、接口字段描述、示例和第三方名称不因本项目MIT许可而获得重新授权。快照保留是为了契约研究、兼容性追溯和测试；仓库未为上游资料声明统一的开源许可。对这些资料做独立再分发时，应核对相应来源条款。

本项目不包含旧版数GB的资源包、游戏图片/字体/HTML模板全集、真实账号、旧数据库或用户运行配置。千寻、米哈游、HoYoverse、Enka等名称仅用于描述接入对象；相关产品、商标及服务由各自权利人持有。

## 随附模块许可

| 模块 | 版本 | 随附许可及声明 |
|---|---|---|
| github.com/beorn7/perks | v1.0.1 | [LICENSE](licenses/github.com_beorn7_perks--LICENSE) |
| github.com/cespare/xxhash/v2 | v2.3.0 | [LICENSE.txt](licenses/github.com_cespare_xxhash_v2--LICENSE.txt) |
| github.com/coder/websocket | v1.8.15 | [LICENSE.txt](licenses/github.com_coder_websocket--LICENSE.txt) |
| github.com/fsnotify/fsnotify | v1.9.0 | [LICENSE](licenses/github.com_fsnotify_fsnotify--LICENSE) |
| github.com/go-chi/chi/v5 | v5.5.0 | [LICENSE](licenses/github.com_go-chi_chi_v5--LICENSE) |
| github.com/go-viper/mapstructure/v2 | v2.4.0 | [LICENSE](licenses/github.com_go-viper_mapstructure_v2--LICENSE) |
| github.com/jackc/pgpassfile | v1.0.0 | [LICENSE](licenses/github.com_jackc_pgpassfile--LICENSE) |
| github.com/jackc/pgservicefile | v0.0.0-20240606120523-5a60cdf6a761 | [LICENSE](licenses/github.com_jackc_pgservicefile--LICENSE) |
| github.com/jackc/pgx/v5 | v5.11.0 | [LICENSE](licenses/github.com_jackc_pgx_v5--LICENSE) |
| github.com/jackc/puddle/v2 | v2.2.3 | [LICENSE](licenses/github.com_jackc_puddle_v2--LICENSE) |
| github.com/mfridman/interpolate | v0.0.2 | [LICENSE.txt](licenses/github.com_mfridman_interpolate--LICENSE.txt) |
| github.com/munnerz/goautoneg | v0.0.0-20191010083416-a7dc8b61c822 | [LICENSE](licenses/github.com_munnerz_goautoneg--LICENSE) |
| github.com/pelletier/go-toml/v2 | v2.2.4 | [LICENSE](licenses/github.com_pelletier_go-toml_v2--LICENSE) |
| github.com/pressly/goose/v3 | v3.29.0 | [LICENSE](licenses/github.com_pressly_goose_v3--LICENSE) |
| github.com/prometheus/client_golang | v1.25.0 | [LICENSE](licenses/github.com_prometheus_client_golang--LICENSE) · [NOTICE](licenses/github.com_prometheus_client_golang--NOTICE) |
| github.com/prometheus/client_model | v0.6.3 | [LICENSE](licenses/github.com_prometheus_client_model--LICENSE) · [NOTICE](licenses/github.com_prometheus_client_model--NOTICE) |
| github.com/prometheus/common | v0.72.0 | [LICENSE](licenses/github.com_prometheus_common--LICENSE) · [NOTICE](licenses/github.com_prometheus_common--NOTICE) |
| github.com/prometheus/procfs | v0.22.0 | [LICENSE](licenses/github.com_prometheus_procfs--LICENSE) · [NOTICE](licenses/github.com_prometheus_procfs--NOTICE) |
| github.com/redis/go-redis/v9 | v9.23.0 | [LICENSE](licenses/github.com_redis_go-redis_v9--LICENSE) |
| github.com/sagikazarmark/locafero | v0.11.0 | [LICENSE](licenses/github.com_sagikazarmark_locafero--LICENSE) |
| github.com/sethvargo/go-retry | v0.5.0 | [LICENSE](licenses/github.com_sethvargo_go-retry--LICENSE) |
| github.com/sourcegraph/conc | v0.3.1-0.20240121214520-5f936abd7ae8 | [LICENSE](licenses/github.com_sourcegraph_conc--LICENSE) |
| github.com/spf13/afero | v1.15.0 | [LICENSE.txt](licenses/github.com_spf13_afero--LICENSE.txt) |
| github.com/spf13/cast | v1.10.0 | [LICENSE](licenses/github.com_spf13_cast--LICENSE) |
| github.com/spf13/pflag | v1.0.10 | [LICENSE](licenses/github.com_spf13_pflag--LICENSE) |
| github.com/spf13/viper | v1.21.0 | [LICENSE](licenses/github.com_spf13_viper--LICENSE) |
| github.com/subosito/gotenv | v1.6.0 | [LICENSE](licenses/github.com_subosito_gotenv--LICENSE) |
| go.uber.org/atomic | v1.12.0 | [LICENSE.txt](licenses/go.uber.org_atomic--LICENSE.txt) |
| go.uber.org/multierr | v1.11.0 | [LICENSE.txt](licenses/go.uber.org_multierr--LICENSE.txt) |
| go.yaml.in/yaml/v3 | v3.0.5 | [LICENSE](licenses/go.yaml.in_yaml_v3--LICENSE) · [NOTICE](licenses/go.yaml.in_yaml_v3--NOTICE) |
| golang.org/x/mod | v0.42.0 | [LICENSE](licenses/golang.org_x_mod--LICENSE) |
| golang.org/x/sync | v0.24.0 | [LICENSE](licenses/golang.org_x_sync--LICENSE) |
| golang.org/x/sys | v0.49.0 | [LICENSE](licenses/golang.org_x_sys--LICENSE) |
| golang.org/x/text | v0.43.0 | [LICENSE](licenses/golang.org_x_text--LICENSE) |
| google.golang.org/protobuf | v1.36.12 | [LICENSE](licenses/google.golang.org_protobuf--LICENSE) |
