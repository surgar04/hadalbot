# 字标生成记录

工具：内置ImageGen，基于用户选定图片进行编辑，开启transparent_background。不是用系统字体重新排字或手工近似绘制。

输入：[选定方案](hadalbot-selected-reference.png)。编辑得到1981×793的透明PNG，正式导出裁切到1898×412；裁切区域RGBA像素逐字节核对一致。只去除外围透明留白，没有拉伸或重排字形。

正式资产使用的编辑提示词：

```text
Edit the attached selected HadalBot logo into a final production wordmark asset. The attached image is the exact approved design; preserve the lettering faithfully. This is precise logo cleanup and background extraction, NOT a redesign. Keep exact text "HadalBot", all eight letters with uppercase H and B, the same rounded geometric letterforms, proportions, optical spacing, circular counters, and the two light mint diagonal pill-shaped accents within the two a letters. Preserve the distinctive letter shapes and the original composition. Remove the entire ivory/off-white background and make it genuinely alpha transparent, including all letter counters and the diagonal negative-space cuts. Clean away incidental paper grain and mottling for flat, crisp, vector-like solid fills: main lettering jade/mint green #238B78 and the two small accents light mint #78C7AA. Do not add strokes, shadows, glow, white fringes, frames, new symbols, or extra text. Do not replace the custom letters with a generic font. Output one standalone horizontal transparent logo, tightly framed around the actual lettering with modest consistent transparent padding; target canvas approximately 2048 x 512, wordmark spanning about 95% of the width, natural original aspect ratio around 5.2:1. Do not stretch or squash the letters. Keep the entire wordmark and accents uncropped.
```

提示词中的尺寸和色值是生成目标，实际交付尺寸以PNG元数据为准。当前资产是位图，仍可能有细微色彩变化，不应将其当作已经完成的纯色矢量稿。
