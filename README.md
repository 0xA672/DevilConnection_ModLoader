# DevilConnection_ModLoader  
**《でびるコネクショん》（恶魔连结）通用模组加载器**

本项目是基于 [ShiroNeko 原始版本](https://steamcommunity.com/app/3054820/discussions/0/671726388306530312/) 的深度重构与增强版。

---

## 📖 项目简介

`DevilConnection_ModLoader` 是专为游戏 **《でびるコネクショん》** 设计的模组加载引擎。  
它能够在 **不修改原版游戏文件** 的前提下，于运行时动态替换或注入自定义内容，实现高度灵活的模组支持。

---

## ✨ 核心特性

- **模组无感加载**  
  自动拦截并映射 `resources/plugins` 目录下的文件夹或 `.asar` 归档文件，无需手动干预。

- **动态解密引擎**  
  支持基于 **RSA + AES** 的资源加密方案，安全加载加密模组资源。

- **脚本动态注入**  
  自动检测并执行 `plugins/xxx/hook.js` 脚本，允许对游戏逻辑进行深度定制与扩展。

- **插件加载优先级机制**  
  `plugins` 目录中的模组按加载顺序分配优先级 (数字越小优先级越高) 。若多个模组包含同名文件，系统将优先使用 **优先级靠前(加载序号小)** 的模组内容。  
  您可通过查看 `resources/mod_loader.log` 日志确认实际加载顺序，请确保游戏本体 `app.bak.asar` 是 **最后加载(优先级靠后)** 的，例如：

> [信息] 扫描完成, 已加载 2 个模组.
> 
> [信息] 优先级 [1]: 心声助手.asar
> 
> [信息] 优先级 [2]: app.bak.asar

---

## ⚖️ 许可与版权声明

- **二改作者**：逍婉瑶  
- **原始版本作者**：[ShiroNeko](https://steamcommunity.com/app/3054820/discussions/0/671726388306530312/)  

> **Copyright (c) 2026, 逍婉瑶. All rights reserved.**
> 
> **Portions Copyright (c) ShiroNeko.**

---

## 🚩 免责声明

- **非官方工具**：本项目为粉丝制作的第三方工具，与原游戏开发商无任何关联。  
- **风险自担**：由于涉及代码混淆、资源替换及底层注入，使用本工具可能导致存档损坏、环境冲突等问题，请务必自行备份。  
- **合规使用**：请确保在遵守当地法律法规及游戏用户协议的前提下使用本工具。

---

## 📅 更新日志

### v2026-01-19
- ✅ **[重构]** 首次正式发布 `DevilConnection_ModLoader`。  
- 🔗 **[兼容]** 完整支持原作者 ShiroNeko 的补丁加载机制。

> 💬 **反馈建议**：如遇 Bug 或有功能建议，欢迎通过 [Issues](https://github.com/shouennyou/DevilConnection_ModLoader/issues) 提交！