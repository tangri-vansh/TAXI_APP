# Safe Mode route evaluation

**Run date:** 2026-10-26
**Risk source:** `boston_pd`
**Method:** Valhalla road routes with the current Boston risk-zone build; Safe Mode uses the configured descending beta schedule and the 20% ETA cap.

| Route pair | Normal ETA (s) | Selected ETA (s) | ETA change | Normal risk exposure | Selected risk exposure | Risk reduction | Result |
|---|---:|---:|---:|---:|---:|---:|---|
| Boston Common → Fenway | 341.808 | 341.808 | 0.00% | 47.7366 | 47.7366 | 0.00% | Normal fallback; no lower-risk alternative fit the detour limit |
| South End → Charlestown | 519.098 | 540.713 | +4.16% | 79.8967 | 76.6411 | 4.07% | Safe Mode selected |
| Roxbury → North End | 427.317 | 401.299 | −6.09% | 38.6838 | 37.5139 | 3.02% | Safe Mode selected |
| East Boston → Back Bay | 517.311 | 517.311 | 0.00% | 57.4628 | 57.4628 | N/A | Normal fallback; normal route risk coverage was incomplete |
| Dorchester → Seaport | 375.893 | 422.088 | +12.29% | 41.6708 | 41.0636 | 1.46% | Safe Mode selected |

Safe Mode found a lower-exposure route for three of the five pairs, all within the 20% ETA cap. One pair fell back because no lower-risk alternative fit the cap; another fell back because the normal route had incomplete risk-zone coverage. The selected routes reduced measured exposure by 1.46–4.07% for the three successful pairs. These results show the routing and fallback logic working, but the measured reductions are modest; they do not establish that a route is safe or that the Boston risk model predicts personal safety.

Reproduce the run with `go run ./cmd/evaluate-safe-mode` from `backend` after setting `DATABASE_URL` and `VALHALLA_URL`. The fixed coordinates and CSV output are defined by that command.
