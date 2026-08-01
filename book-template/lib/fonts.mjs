// 사이트 head와 PDF 표지·차례가 같은 웹폰트를 쓴다.
// 한글 폰트가 없는 CI 러너에서도 본문이 명조·고딕으로 렌더링되게 하는 폴백이다.
export const FONT_URL =
  'https://fonts.googleapis.com/css2?family=Noto+Sans+KR:wght@400;500;600;700&family=Noto+Serif+KR:wght@400;600;700&display=swap'

export const FONT_LINKS = `
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link rel="stylesheet" href="${FONT_URL}">`

// 표지 전용 폰트. 네이버 D2Coding(OFL 1.1)을 한자 없이 서브셋해 각 책의
// public/fonts/에 둔다(출처와 재생성 방법은 그 디렉터리의 README.md).
// 홈의 표지 카드와 PDF 표지가 이 선언 하나를 함께 쓴다.
// 폰트를 실제로 내려받는 것은 표지가 있는 홈뿐이다(본문 장에는 이 글꼴을 쓰는 요소가 없다).
export const COVER_FONT =
  "'D2Coding', ui-monospace, SFMono-Regular, Menlo, Consolas, monospace"

export function coverFontFaceCss(base) {
  return ['Regular', 'Bold']
    .map(
      (w) => `@font-face {
  font-family: 'D2Coding';
  src: url('${base}fonts/D2Coding-${w}.woff2') format('woff2');
  font-weight: ${w === 'Bold' ? 700 : 400};
  font-style: normal;
  font-display: swap;
}`,
    )
    .join('\n')
}
