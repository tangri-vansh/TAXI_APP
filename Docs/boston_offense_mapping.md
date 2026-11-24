# Boston offense code mapping (draft)

**Status:** Draft mapping per user direction: retain clear offenses and exclude all uncertain or out-of-taxonomy rows. The importer uses the code-based mapping in `backend/data/boston_offense_mapping.csv`; excluded rows are not stored or scored.

## Snapshot and rules

- Source: `tmpivat4nah.csv`; 305,787 rows with occurrence dates from 2023-01-01 through 2026-10-19.
- 123 distinct code-description pairs. `OFFENSE_CODE_GROUP` and `UCR_PART` are empty in this snapshot.
- Mapping is keyed by `OFFENSE_CODE`; code 111 has two spelling variants, both treated as the same offense.
- Weights reuse the agreed design scale: violent offenses 1.0, harassment/threats 0.8, theft 0.5, burglary/property damage 0.3. Robbery is 1.0; vehicle theft is 0.5.
- The source has no distinct carjacking code. Ordinary auto theft is not labeled carjacking.
- Traffic incidents, investigations, medical/service calls, drug offenses, mixed weapon codes, vague disputes, recoveries, and other uncertain offenses are excluded.
- Boston notes that police data may be preliminary and change over time; each import records its snapshot date. [Boston Police data notes](https://www.boston.gov/departments/analytics-team/data-boston-police-department)

## Included category weights

| Category | Weight | Rows |
|---|---:|---:|
| `violent_person_offense` | 1.0 | 18,393 |
| `robbery` | 1.0 | 2,724 |
| `harassment_threats` | 0.8 | 11,411 |
| `theft` | 0.5 | 38,947 |
| `vehicle_theft` | 0.5 | 4,833 |
| `burglary` | 0.3 | 3,646 |
| `property_damage` | 0.3 | 10,330 |

**Snapshot disposition:** 90,284 rows retained by the mapping and 215,503 excluded (total 305,787). Coordinate validation is separate; rows without valid coordinates are skipped by the spatial importer.

## Code-level inventory

| Offense code | Description | Rows | Category | Weight | Status | Rationale |
|---:|---|---:|---|---:|---|---|
| 111 | MURDER, NON-NEGLIGENT MANSLAUGHTER | 99 | `violent_person_offense` | 1.0 | INCLUDED | Person-directed violence, homicide, kidnapping, or affray; existing violent-offense weight. |
| 111 | MURDER, NON-NEGLIGIENT MANSLAUGHTER | 1 | `violent_person_offense` | 1.0 | INCLUDED | Person-directed violence, homicide, kidnapping, or affray; existing violent-offense weight. |
| 121 | MANSLAUGHTER - VEHICLE - NEGLIGENCE | 7 | — | 0 | EXCLUDED | Vehicle manslaughter is excluded with traffic incidents. |
| 122 | MANSLAUGHTER - NEGLIGENCE | 2 | `violent_person_offense` | 1.0 | INCLUDED | Person-directed violence, homicide, kidnapping, or affray; existing violent-offense weight. |
| 301 | ROBBERY | 2,724 | `robbery` | 1.0 | INCLUDED | Robbery involves force or threat; reuses the violent-offense weight. The source does not identify carjacking separately. |
| 423 | ASSAULT - AGGRAVATED | 6,026 | `violent_person_offense` | 1.0 | INCLUDED | Person-directed violence, homicide, kidnapping, or affray; existing violent-offense weight. |
| 520 | BURGLARY - RESIDENTIAL | 2,170 | `burglary` | 0.3 | INCLUDED | Burglary, vehicle break-in, or possession of burglary tools; existing burglary weight. |
| 530 | BREAKING AND ENTERING (B&E) MOTOR VEHICLE | 58 | `burglary` | 0.3 | INCLUDED | Burglary, vehicle break-in, or possession of burglary tools; existing burglary weight. |
| 531 | BREAKING AND ENTERING (B&E) MOTOR VEHICLE (NO PROPERTY STOLEN) | 37 | `burglary` | 0.3 | INCLUDED | Burglary, vehicle break-in, or possession of burglary tools; existing burglary weight. |
| 540 | BURGLARY - COMMERICAL | 1,370 | `burglary` | 0.3 | INCLUDED | Burglary, vehicle break-in, or possession of burglary tools; existing burglary weight. |
| 611 | LARCENY PICK-POCKET | 247 | `theft` | 0.5 | INCLUDED | Larceny offenses; existing theft weight. |
| 612 | LARCENY PURSE SNATCH - NO FORCE | 73 | `theft` | 0.5 | INCLUDED | Larceny offenses; existing theft weight. |
| 613 | LARCENY SHOPLIFTING | 14,424 | `theft` | 0.5 | INCLUDED | Larceny offenses; existing theft weight. |
| 614 | LARCENY THEFT FROM MV - NON-ACCESSORY | 6,858 | `theft` | 0.5 | INCLUDED | Larceny offenses; existing theft weight. |
| 615 | LARCENY THEFT OF MV PARTS & ACCESSORIES | 1,035 | `theft` | 0.5 | INCLUDED | Larceny offenses; existing theft weight. |
| 616 | LARCENY THEFT OF BICYCLE | 2,214 | `theft` | 0.5 | INCLUDED | Larceny offenses; existing theft weight. |
| 617 | LARCENY THEFT FROM BUILDING | 7,514 | `theft` | 0.5 | INCLUDED | Larceny offenses; existing theft weight. |
| 618 | LARCENY THEFT FROM COIN-OP MACHINE | 8 | `theft` | 0.5 | INCLUDED | Larceny offenses; existing theft weight. |
| 619 | LARCENY ALL OTHERS | 6,574 | `theft` | 0.5 | INCLUDED | Larceny offenses; existing theft weight. |
| 706 | AUTO THEFT - MOTORCYCLE / SCOOTER | 1,194 | `vehicle_theft` | 0.5 | INCLUDED | Vehicle theft; reuses the theft weight and is not labeled as carjacking. |
| 724 | AUTO THEFT | 3,385 | `vehicle_theft` | 0.5 | INCLUDED | Vehicle theft; reuses the theft weight and is not labeled as carjacking. |
| 727 | AUTO THEFT - LEASED/RENTED VEHICLE | 254 | `vehicle_theft` | 0.5 | INCLUDED | Vehicle theft; reuses the theft weight and is not labeled as carjacking. |
| 735 | RECOVERED - MV RECOVERED IN BOSTON (STOLEN OUTSIDE BOSTON) | 832 | — | 0 | EXCLUDED | Recovered vehicle; recovery location is not the theft location. |
| 736 | RECOVERED - MV RECOVERED IN BOSTON (STOLEN IN BOSTON) MUST BE SUPPLEMENTAL | 100 | — | 0 | EXCLUDED | Supplemental recovered-vehicle record; recovery location is not the theft location. |
| 801 | ASSAULT - SIMPLE | 12,110 | `violent_person_offense` | 1.0 | INCLUDED | Person-directed violence, homicide, kidnapping, or affray; existing violent-offense weight. |
| 900 | ARSON | 82 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 990 | Justifiable Homicide | 1 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 1001 | FORGERY / COUNTERFEITING | 1,259 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 1102 | FRAUD - FALSE PRETENSE / SCHEME | 3,730 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 1106 | FRAUD - CREDIT CARD / ATM FRAUD | 1,847 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 1107 | FRAUD - IMPERSONATION | 802 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 1108 | FRAUD - WELFARE | 183 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 1109 | FRAUD - WIRE | 1,001 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 1201 | EMBEZZLEMENT | 190 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 1300 | STOLEN PROPERTY - BUYING / RECEIVING / POSSESSING | 642 | — | 0 | EXCLUDED | Possession location may differ from the original theft location. |
| 1304 | PROPERTY - STOLEN THEN RECOVERED | 151 | — | 0 | EXCLUDED | Recovery record, not a new theft incident. |
| 1402 | VANDALISM | 10,182 | `property_damage` | 0.3 | INCLUDED | Vandalism and graffiti; reuses the existing burglary/property weight. |
| 1415 | GRAFFITI | 148 | `property_damage` | 0.3 | INCLUDED | Vandalism and graffiti; reuses the existing burglary/property weight. |
| 1501 | WEAPON VIOLATION - CARRY/ POSSESSING/ SALE/ TRAFFICKING/ OTHER | 1,163 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 1601 | PROSTITUTION | 2 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 1602 | PROSTITUTION - SOLICITING | 16 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 1603 | PROSTITUTION - ASSISTING OR PROMOTING | 1 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 1810 | DRUGS - POSSESSION/ SALE/ MANUFACTURING/ USE | 8,468 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 1825 | DRUGS - POSSESSION OF DRUG PARAPHANALIA | 76 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 1831 | SICK ASSIST | 28,736 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 1832 | SICK ASSIST - DRUG RELATED ILLNESS | 3,153 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 1903 | GAMBLING - EQUIP VIOLATIONS | 1 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 2101 | OPERATING UNDER THE INFLUENCE (OUI) ALCOHOL | 360 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 2102 | OPERATING UNDER THE INFLUENCE (OUI) DRUGS | 22 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 2204 | LIQUOR LAW VIOLATION | 108 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 2401 | AFFRAY | 119 | `violent_person_offense` | 1.0 | INCLUDED | Person-directed violence, homicide, kidnapping, or affray; existing violent-offense weight. |
| 2403 | DISTURBING THE PEACE/ DISORDERLY CONDUCT/ GATHERING CAUSING ANNOYANCE/ NOISY PAR | 731 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 2604 | EXTORTION OR BLACKMAIL | 514 | — | 0 | EXCLUDED | Extortion/blackmail does not have a clear fit in the existing categories. |
| 2608 | CHILD REQUIRING ASSISTANCE (FOMERLY CHINS) | 24 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 2610 | TRESPASSING | 1,792 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 2612 | FIRE REPORT/ALARM - FALSE | 167 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 2613 | ANIMAL ABUSE | 86 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 2616 | POSSESSION OF BURGLARIOUS TOOLS | 11 | `burglary` | 0.3 | INCLUDED | Burglary, vehicle break-in, or possession of burglary tools; existing burglary weight. |
| 2618 | EXPLOSIVES - POSSESSION OR USE | 6 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 2619 | FUGITIVE FROM JUSTICE | 160 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 2622 | KIDNAPPING/CUSTODIAL KIDNAPPING/ ABDUCTION | 36 | `violent_person_offense` | 1.0 | INCLUDED | Person-directed violence, homicide, kidnapping, or affray; existing violent-offense weight. |
| 2628 | OBSCENE PHONE CALLS | 5 | `harassment_threats` | 0.8 | INCLUDED | Harassment, bodily-harm threats, obscene calls, or witness intimidation; existing harassment weight. |
| 2632 | EVADING FARE | 93 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 2642 | TRUANCY / RUNAWAY | 55 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 2646 | LIQUOR/ALCOHOL - DRINKING IN PUBLIC | 327 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 2647 | THREATS TO DO BODILY HARM | 6,782 | `harassment_threats` | 0.8 | INCLUDED | Harassment, bodily-harm threats, obscene calls, or witness intimidation; existing harassment weight. |
| 2648 | BOMB THREAT | 91 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 2657 | VIOLATION - CITY ORDINANCE | 391 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 2660 | OTHER OFFENSE | 129 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 2662 | BALLISTICS EVIDENCE/FOUND | 743 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 2670 | HARASSMENT/ CRIMINAL HARASSMENT | 4,562 | `harassment_threats` | 0.8 | INCLUDED | Harassment, bodily-harm threats, obscene calls, or witness intimidation; existing harassment weight. |
| 2905 | VAL - VIOLATION OF AUTO LAW | 4,802 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 2907 | VAL - OPERATING AFTER REV/SUSP. | 1,513 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 2914 | VAL - OPERATING W/O AUTHORIZATION LAWFUL | 243 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3001 | DEATH INVESTIGATION | 862 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3004 | INJURY BICYCLE NO M/V INVOLVED | 82 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3006 | SICK/INJURED/MEDICAL - PERSON | 3,858 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3007 | SUDDEN DEATH | 2,008 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3008 | SUICIDE / SUICIDE ATTEMPT | 158 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3016 | FIREARM/WEAPON - ACCIDENTAL INJURY / DEATH | 15 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3018 | SICK/INJURED/MEDICAL - POLICE | 1,341 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3029 | PRISONER - SUICIDE / SUICIDE ATTEMPT | 26 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3106 | PROPERTY - ACCIDENTAL DAMAGE | 985 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3108 | FIRE REPORT | 1,564 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3110 | SERVICE TO OTHER AGENCY | 1,883 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3111 | LICENSE PREMISE VIOLATION | 1,182 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3112 | LANDLORD - TENANT | 1,354 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3114 | INVESTIGATE PROPERTY | 13,350 | — | 0 | EXCLUDED | Investigation-only record without a specific offense classification. |
| 3115 | INVESTIGATE PERSON | 30,901 | — | 0 | EXCLUDED | Investigation-only record without a specific offense classification. |
| 3116 | HARBOR INCIDENT / VIOLATION | 197 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3119 | FIREARM/WEAPON - FOUND OR CONFISCATED | 704 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3122 | AIRCRAFT INCIDENTS | 77 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3123 | EXPLOSIVES - TURNED IN OR FOUND | 2 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3125 | WARRANT ARREST - BOSTON WARRANT (MUST BE SUPPLEMENTAL) | 281 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3126 | WARRANT ARREST - OUTSIDE OF BOSTON WARRANT | 1,922 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3130 | SEARCH WARRANT | 465 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3170 | INTIMIDATING WITNESS | 62 | `harassment_threats` | 0.8 | INCLUDED | Harassment, bodily-harm threats, obscene calls, or witness intimidation; existing harassment weight. |
| 3201 | PROPERTY - LOST/ MISSING | 10,708 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3202 | PROPERTY - LOST THEN LOCATED | 174 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3203 | FIREARM/WEAPON - LOST | 24 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3205 | M/V PLATES - LOST | 584 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3207 | PROPERTY - FOUND | 3,812 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3301 | VERBAL DISPUTE | 7,249 | — | 0 | EXCLUDED | A verbal dispute alone does not establish a crime or physical threat. |
| 3304 | NOISY PARTY/RADIO-NO ARREST | 305 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3305 | DRUNKENNESS | 21 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3402 | ANIMAL INCIDENTS (DOG BITES, LOST DOG, ETC) | 978 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3403 | PROTECTIVE CUSTODY / SAFEKEEPING | 6 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3410 | TOWED MOTOR VEHICLE | 14,039 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3501 | MISSING PERSON | 853 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3502 | MISSING PERSON - LOCATED | 4,778 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3503 | MISSING PERSON - NOT REPORTED - LOCATED | 1,570 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3625 | DANGEROUS OR HAZARDOUS CONDITION | 128 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3801 | M/V ACCIDENT - OTHER | 6,453 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3802 | M/V ACCIDENT - PROPERTY DAMAGE | 6,909 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3803 | M/V ACCIDENT - PERSONAL INJURY | 3,499 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3805 | M/V ACCIDENT - POLICE VEHICLE | 1,035 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3807 | M/V ACCIDENT - OTHER CITY VEHICLE | 1,084 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3810 | M/V ACCIDENT - INVOLVING BICYCLE - INJURY | 953 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3811 | M/V ACCIDENT - INVOLVING BICYCLE - NO INJURY | 411 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3820 | M/V ACCIDENT - INVOLVING PEDESTRIAN - INJURY | 2,186 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3821 | M/V ACCIDENT - INVOLVING PEDESTRIAN - NO INJURY | 510 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3830 | M/V - LEAVING SCENE - PERSONAL INJURY | 1,248 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
| 3831 | M/V - LEAVING SCENE - PROPERTY DAMAGE | 17,969 | — | 0 | EXCLUDED | Outside the retained crime categories or too ambiguous for a consistent risk classification. |
