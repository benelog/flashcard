# 개념 그림 스타일 규약

책의 개념 그림은 전부 draw.io 소스(`*.drawio`)로 두고 PNG로 내보내 `book/public/screenshots/`에 넣는다.
표지 그림의 세 층 색(화면=주황, 로직=시안, 데이터=보라)을 그대로 따른다.

## 색

| 역할 | fillColor | strokeColor | fontColor |
|---|---|---|---|
| 화면·브라우저·사용자 | `#FBE4CC` | `#D79B63` | `#7F5124` |
| 로직·서버·Go 코드 | `#CFE2F3` | `#4A7BA6` | `#1F4E79` |
| 데이터·DB·파일 | `#E4D7F5` | `#8E6BBF` | `#4A2C7A` |
| 외부 서비스(Supabase·Google·GitHub·Vercel 플랫폼) | `#EDEDED` | `#8C8C8C` | `#333333` |
| 판정(마름모)·주의 | `#FFF2CC` | `#B7950B` | `#5C4A00` |
| 성공·허용 | `#D9EAD3` | `#6AA84F` | `#3F6B2B` |
| 실패·금지·오류 | `#F8D7DA` | `#C0392B` | `#7B1F1F` |
| 묶음(컨테이너, 점선) | `none` | `#9AA0A6` | `#5F6368` |

## 글자

- 상자 이름 14px, 상자 안 보조 설명 12px, 그림 제목·구역 이름 15px 굵게, 각주성 메모 12px `#6B6B6B`.
- 코드 식별자(경로·함수·환경 변수)는 `fontFamily=Courier New`.
- 한글 라벨은 원고의 용어를 그대로 쓴다.

## 선

- 기본 화살표: `strokeColor=#4D4D4D;strokeWidth=1.5;endArrow=classic;endFill=1;html=1`.
- 되돌아오는·반복 화살표: 같은 스타일에 `dashed=0`, 라벨로 뜻을 적는다.
- 금지된 경로: `strokeColor=#C0392B;dashed=1;dashPattern=6 4`에 X 표시.
- 보조 지시선: `strokeColor=#9AA0A6;dashed=1;dashPattern=2 3;endArrow=none`.
- 꺾이는 경로는 `edgeStyle=orthogonalEdgeStyle;rounded=1`.

## 크기

- 논리 폭 680px 이하, 높이 460px 이하(두 쪽 펼침에서 한 단에 캡션과 함께 들어가는 크기).
- 상자 모서리 `rounded=1;arcSize=12`, 높이 44px 기본, 두 줄이면 56px.
- 내보내기: `drawio -x -f png -e -b 12 -s 2 -o ../../public/screenshots/<이름>.png <이름>.drawio`

## 소스 규칙

- XML 주석을 쓰지 않는다. `&`·`<`·`>`는 이스케이프한다.
- 모든 edge는 `<mxGeometry relative="1" as="geometry"/>`를 자식으로 둔다.
- `<mxGraphModel adaptiveColors="auto">`로 시작하고 `id="0"`, `id="1"` 셀을 둔다.

## 그리는 절차

1. 원고의 해당 절을 읽어 용어·파일명·함수명을 그대로 쓴다. 원고에 없는 개념을 지어내지 않는다.
2. `<이름>.drawio`를 쓴다. 상자 12개·화살표 14개 이하. 상자 이름은 한 줄, 보조 설명은 12px 한 줄. 문장을 상자에 넣지 않는다.
3. 내보낸 PNG를 열어 글자 넘침·화살표 겹침·라벨 겹침을 눈으로 확인하고, 문제가 있으면 좌표를 고쳐 다시 내보낸다.
4. 원고에는 `fc-shots` 블록과 `그림 N` 캡션으로 넣고 `node scripts/renumber-figures.mjs`로 번호를 매긴다.
