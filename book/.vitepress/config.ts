// 모든 사이트 설정은 book.config.mjs에서 온다. 이 파일은 엔진에 넘기는 심(shim)이다.
import { defineBookConfig } from 'book-template/config'
import book from '../book.config.mjs'

export default defineBookConfig(book)
