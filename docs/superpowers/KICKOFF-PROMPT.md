# Yangi sessiya/AI uchun start-prompt (nusxa ko'chirib ishlating)

> Bu fayl — istalgan AI-vositaga (Claude Code, boshqa vositalar) loyihani davom ettirish uchun berish mumkin bo'lgan tayyor prompt. Pastdagi bloqni to'liq nusxa ko'chirib, yangi sessiyaga joylashtiring. Hech narsa o'zgartirish shart emas — hujjat o'zi joriy holatni ko'rsatadi.

---

```
Bu AvtoTest loyihasi — O'zbekiston YHQ (haydovchilik) imtihoniga tayyorlovchi
pullik onlayn maktab-startap. Go backend + Next.js frontend.

AVVAL O'QI (o'zgartirmasdan): docs/superpowers/2026-07-24-SESSION-HANDOFF.md
Bu hujjat aniq holat (nima tugagan, git/DB versiyasi) va keyingi aniq qadamni
beradi. Undagi bo'lim 5 ("Operatsion faktlar")ga qat'iy amal qil (Go PATH,
sqlc, DB-test flag'lari, dev-server restart usuli va h.k.) — bular vaqtni
tejaydi va xatolarni oldini oladi.

VAZIFA: Hujjatdagi "KEYINGI ANIQ QADAM" bo'limida ko'rsatilgan ishni davom
ettir. Agar o'sha ish uchun spec/reja hujjati (docs/superpowers/specs/,
docs/superpowers/plans/) allaqachon yozilgan va tasdiqlangan bo'lsa —
QAYTA brainstorming/spec/reja QILMA, to'g'ridan-to'g'ri implementatsiyani
davom ettir. Agar YANGI ish bo'lsa (hali spec yo'q) — pastdagi ish uslubidan
boshla.

ISH USLUBI (bu loyihada shu tartib har doim ishlatilgan va yaxshi natija
bergan):
1. Har yangi Plan: avval qisqa brainstorming (loyiha kontekstini o'rganib,
   mavjud o'xshash kod/naqshlarni topib, qisqa dizayn taklif qilib, tasdiq
   olish) → spec hujjat yoz (docs/superpowers/specs/YYYY-MM-DD-<nom>.md,
   commit qil) → implementatsiya rejasi yoz (docs/superpowers/plans/,
   aniq kod-eskizlar bilan, "TBD"/placeholder'siz, commit qil) → TDD bilan
   qur (test yoz → yiqilishini ko'r → implement → o'tishini ko'r) → har
   task alohida commit.
2. Agar ishlatayotgan vositangda subagent/parallel-agent imkoniyati bo'lsa
   (masalan Claude Code'ning Task/Agent vositalari): har taskni alohida
   "implementer" subagent'ga ber, keyin mustaqil "reviewer" subagent bilan
   tekshirtir (spec-muvofiqlik + kod sifati, xato topilsa tuzat va qayta
   tekshirtir). Fayllar jihatidan mustaqil (bir-biriga tegmaydigan)
   task'larni haqiqiy parallel bajarish mumkin (isolated worktree bilan,
   keyin fast-forward yoki cherry-pick bilan asosiy branch'ga qo'shiladi).
   Agar bunday vositalar yo'q bo'lsa — xuddi shu mantiqiy tartibni
   (implement→o'zing qayta ko'rib chiq→tuzat) qo'lda bajar.
3. Pul/entitlement bilan bog'liq kodda (to'lov webhook'lari, VIP berish)
   ALOHIDA EHTIYOT bo'l: yozuvlar bitta DB-tranzaksiyada bo'lishi kerak,
   concurrent-holatlar uchun row-lock (`SELECT ... FOR UPDATE`) va
   idempotentlik tekshiruvlari majburiy. M2-02 (Payme)da bu saboq keyin
   review orqali topilib tuzatilgan edi; M2-03 (Click)da boshidanoq to'g'ri
   qurilib, birinchi urinishdayoq toza o'tgan — shu ikkinchi yondashuvni
   qo'lla.
4. Testlar doim yashil bo'lsin, build/lint/typecheck toza bo'lsin — har
   task oxirida va butun ish tugagach to'liq tekshir (backend: `go build
   ./... && go test ./... -p 1`; frontend: `npm run typecheck && npm run
   lint && npm run test`).
5. Har commit'ni git'ga push qil. Ish (Plan) to'liq tugagach —
   `docs/superpowers/2026-07-24-SESSION-HANDOFF.md`ni YANGILA: nima
   tugaganini yoz, "KEYINGI ANIQ QADAM" bo'limini yangi holatga moslab
   qayta yoz, commit+push qil. Bu hujjat — keyingi har qanday AI/sessiya
   o'qiydigan yagona ishonchli manba, shuning uchun uni doim aniq va
   yangilangan holda saqlash MUHIM.
6. To'xtab qolma, ortiqcha savol berma — faqat haqiqiy noaniqlik yoki
   muhim arxitektura qarori bo'lsa so'ra. Aks holda o'zing qaror qabul
   qilib, sababini spec/commit izohida yoz.
7. Loyihaning to'liq bosqichlar xaritasi (M2→M7, admin oxirida):
   docs/superpowers/2026-07-24-roadmap-m2-to-admin.md — har yangi Plan
   boshlashdan oldin shu yerdan bog'liqliklarni tekshir.

Boshla.
```
