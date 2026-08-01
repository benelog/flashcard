# 표지 폰트

표지(홈 랜딩과 PDF 표지)는 네이버의 **D2Coding**을 쓴다.
본문은 그대로 Noto Serif KR·Noto Sans KR이고, 이 폰트는 표지 안에서만 쓰인다.

- 출처: https://github.com/naver/d2-coding-font
- 라이선스: SIL Open Font License 1.1 (`OFL.txt`)
- 원본 웹폰트: https://naver.github.io/d2-coding-font/fonts/ 의 `D2Coding-Regular.woff2`·`D2Coding-Bold.woff2`

원본은 두 벌 합쳐 3MB다. 표지 한 장을 위해 그만큼 받게 할 수 없으므로 한자를 걷어낸 서브셋을 두었다(합쳐 912KB).
한글은 완성형 전체(U+AC00–D7A3)를 남겨 두었으므로 `book.config.mjs`의 표지 문구를 바꿔도 글자가 빠지지 않는다.

재생성은 fonttools(`pyftsubset`)로 한다. 원본 두 벌을 내려받은 디렉터리에서:

```sh
U="U+0020-007E,U+00A0-00FF,U+2000-206F,U+20A0-20BF,U+2022,U+00B7,U+2013,U+2014,U+2018-201F,U+1100-11FF,U+3130-318F,U+AC00-D7A3,U+FF01-FF60"
for w in Regular Bold; do
  pyftsubset D2Coding-$w.woff2 --unicodes="$U" --layout-features='*' \
    --flavor=woff2 --output-file=D2Coding-$w.woff2
done
```

`@font-face` 선언은 `book-template/lib/fonts.mjs`에 있다. 사이트 head와 PDF 표지가 같은 선언을 쓴다.
