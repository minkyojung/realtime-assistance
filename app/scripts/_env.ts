/**
 * .env 는 저장소 루트에 있다 (app/ 이 아니라).
 * 모든 스크립트가 이 파일을 가장 먼저 import 해서
 * 쉘에서 source 했는지 여부와 무관하게 같은 값을 읽도록 한다.
 *
 * 이걸 빠뜨리면 DATABASE_URL 이 비어 pg 가 로컬 기본 DB로 붙고,
 * "relation ... does not exist" 같은 엉뚱한 오류가 난다.
 */
import { config } from 'dotenv'
import { join, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

config({ path: join(dirname(fileURLToPath(import.meta.url)), '..', '..', '.env'), quiet: true })
