# HadalBot 字标

采用用户选定的薄荷绿圆润字标。完整名称为HadalBot，H和B保持大写；两个a中的浅薄荷色斜向细节是识别元素，应与主体一起保留。

## 正式资产

| 文件 | 说明 |
|---|---|
| [hadalbot-wordmark.png](../assets/hadalbot-wordmark.png) | 1898 × 412，RGBA透明PNG，当前README使用此版本 |
| [选定设计参考](hadalbot-selected-reference.png) | 用户确认的原始方案，保留作后续编辑与视觉核对依据 |
| [浅色/深色背景预览](hadalbot-wordmark-backgrounds.png) | 520px显示宽度下的背景适配核对图 |

字标由图像生成工具基于选定方案进行透明背景编辑；正式导出仅裁去外围透明留白，保留字形像素和宽高比例。当前交付为位图字标，不是可编辑字体或矢量文件，无需加载外部字体。

## 使用方式

```html
<img src="docs/assets/hadalbot-wordmark.png" width="520" alt="HadalBot" />
```

- README使用520px显示宽度，高度按原比例自适应，不拉伸、不压扁。
- 主体为薄荷玉绿色，细节为浅薄荷绿色；保留两个a的斜向留白和点缀。
- 周围留出空间，不紧贴边框、文字或其他图形，不额外添加阴影、描边和背景色块。
- 透明文件可放在浅色和深色背景；使用前检查实际显示对比度。
- 原docs/assets/hadalbot.svg保留作历史图标，当前README不再引用它。

导出核对：包含真实透明通道，四角alpha均为0；裁切区域内RGBA像素与生成源一致。浅色与深色预览已目视检查，完整名称及浅色细节均可辨认。[生成记录](generation.md)保留所用工具和提示词。
