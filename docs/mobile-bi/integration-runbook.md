# 本地联调与验证

所有联调使用隔离 PostgreSQL，不连接生产。后端 integration fixture 使用真实 migration、repository、Go HTTP handlers 和实际小程序 TypeScript client，仅微信兑换使用测试替身；总结测试使用测试模型。

```sh
cd backend
BI_MOBILE_PROJECT=/path/to/devku_mobile/miniprogram go test -tags=integration ./internal/bi -count=1
 go test -tags=integration ./internal/repository -run 'Desktop.*(Native|Period|Report)|BIUpgrade' -count=1
 go test -tags=unit ./...
 go test -tags=integration ./...
 make build
 golangci-lint run ./...
```

`BI_MOBILE_PROJECT` 指向小程序工程；本地兄弟目录可自动发现。CI 没有该 checkout 时实际 TypeScript 联调会明确 skip，应单独记录，不能替代为模拟 transport 验收。

```sh
cd frontend
pnpm typecheck
pnpm test:run
pnpm lint:check
pnpm build

cd /path/to/devku_mobile/miniprogram
npm run check
```

小程序 `tests/native-http.ts` 只允许 loopback HTTP fixture，验证独立登录、无企业授权、邀请预览与领取、企业列表、精确用量、总结读取与复制、跨企业拒绝和退出。不要保存真实 refresh token 或邀请码到脚本。

H5 默认是独立演示，可使用 `npm run dev:h5` 在 127.0.0.1:10086 预览。H5 页面通过不等同微信模拟器、真机、容量或生产接入验收；分别记录。
