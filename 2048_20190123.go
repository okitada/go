/*
2048.go - 2048 Game

2017/01/07 pprof効果がわかるようにgetGapからlevel=0を別関数に分離(getGap1)
2017/01/21 memprofile オプション追加 2048.exe -memprofile=6060 で実行し、http://localhost:6060/debug/pprof/heap?debug=1 を開く
2017/01/24 getGapをチューニング（appear途中で最大値を超えたら枝刈りで読み中断）
2017/01/27 getGap,getGap1をチューニング（appear前のGapを1度計算しておいてから、各appearによる差分のみを加算）
2017/02/11 calcGapをチューニング（端と端以外のGap計算時に端の方が小さければGapを増やす。-calc_gap_mode追加）
2017/02/12 D_BONUS_POINT_MAX, D_BONUS2廃止

Game Over! (level=4 seed=10) 2017/02/12 13:31:45 #10 Ave.=67530 Max=121580(seed=10)
getGap=312196223 calcGap=6188443217 1,10.0,1.1,0.0 55%,1 30000,1 10%,1 200000,1 1 calc_gap_mode=3
[4530] 121580 (0.0/987.5 sec) 95312506.000000 2017/02/12 13:31:45 seed=10 2=75.23%
    2     8   256    16
   32   512   128  2048
   16    64    16     2
    8  8192     2     4

Game Over! (level=4 seed=40) 2017/02/15 23:51:10 #1 Ave.=135260 Max=135260(seed=40) Min=135260(seed=40)
getGap=455717971 calcGap=9284335604 10.0,0.0 55%,1 20000,1 10%,1 200000,1 1 calc_gap_mode=5
[1:5130] 135260 (0.02/1536.3 sec) 79687503.691162 2017/02/15 23:51:10 seed=40 2=74.91% Ave.=270520
    2    32    64   128
    4   256  1024     8
    8    32  2048     4
    4     8  1024  8192
Total time = 1536.3 (sec)

Game Over! (level=4 seed=10) 2017/02/17 00:22:17 #10 Ave.=59322 Max=134208(seed=9) Min=26424(seed=1)
getGap=75228566 calcGap=1389053290 10.0,0.0 55%,1 20000,1 10%,1 200000,1 1 calc_gap_mode=3
[10:1624] 34864 (0.02/264.1 sec) 79687522.076172 2017/02/17 00:22:17 seed=10 2=74.98% Ave.=62808
 2048    16     4     2
 1024   512    64     8
   32   256    16     2
    2     8    64     4
Total time = 6509.7 (sec)

*/

package main

import "flag"
import "fmt"
import "math/rand"
import "time"
import "os"
import "log"
import "runtime/pprof"
import "net/http"
import _ "net/http/pprof"

var auto_mode int = 4 // >=0 depth
var calc_gap_mode int = 0 // gap計算モード(0:normal 1:端の方が小さければ+1 2:*2 3:+大きい方の値 4:+大きい方の値/10 5:+両方の値)
var print_mode int = 100 // 途中経過の表示間隔(0：表示しない)
var print_mode_turbo int = 1
var pause_mode int = 0
var one_time int = 1 // 繰り返し回数
var seed int64 = 1
var turbo_minus_percent       = 55
var turbo_minus_percent_level = 1
var turbo_minus_score         = 20000
var turbo_minus_score_level   = 1
var turbo_plus_percent        = 10
var turbo_plus_percent_level  = 1
var turbo_plus_score          = 200000
var turbo_plus_score_level    = 1

const D_BONUS = 10
const D_BONUS_USE_MAX = true //10固定ではなく最大値とする
const GAP_EQUAL = 0

const INIT2 = 1
const INIT4 = 2
const RNDMAX = 4
const GAP_MAX = 100000000.0
const XMAX = 4
const YMAX = 4
const XMAX_1 = (XMAX-1)
const YMAX_1 = (YMAX-1)

var board [XMAX][YMAX]int
var sp int = 0

var pos_x[XMAX*YMAX] int
var pos_y[XMAX*YMAX] int
var pos_val[XMAX*YMAX] int
var score int
var gen int
var count_2 int = 0
var count_4 int = 0
var count_calcGap uint64 = 0
var count_getGap uint64 = 0

var start_time int64
var last_time int64
var total_start_time int64
var total_last_time int64

var count int = 1
var sum_score int = 0
var max_score int = 0
var max_seed int64 = 0
var min_score int = int(GAP_MAX)
var min_seed int64 = 0
var ticks_per_sec float64 = 1000000000

func main() {
	pauto_mode := flag.Int("level", auto_mode, "読みの深さ(>0)")
	pcalc_gap_mode := flag.Int("calc", calc_gap_mode, "gap計算モード(0:normal 1:端の方が小さければ+1 2:*2 3:+大きい方の値 4:+大きい方の値/10 5:+両方の値)")
	pprint_mode := flag.Int("print", print_mode, "途中経過の表示間隔(0：表示しない)")
	pprint_mode_turbo := flag.Int("print_mode_turbo", print_mode_turbo, "0:PRINT_MODEに従う 1:TURBO_MINUS_SCOREを超えたら強制表示 2:TURBO_PLUS_SCOREを超えたら強制表示")
	ppause_mode := flag.Int("pause", pause_mode, "終了時に一時中断(0/1)")
	pone_time := flag.Int("one_time", one_time, "N回で終了")
	pseed := flag.Int64("seed", seed, "乱数の種")
	pturbo_minus_percent := flag.Int("turbo_minus_percent", turbo_minus_percent, "turbo_minus_percent")
	pturbo_minus_percent_level := flag.Int("turbo_minus_percent_level", turbo_minus_percent_level, "turbo_minus_percent_level")
	pturbo_minus_score := flag.Int("turbo_minus_score", turbo_minus_score, "turbo_minus_score")
	pturbo_minus_score_level := flag.Int("turbo_minus_score_level", turbo_minus_score_level, "turbo_minus_score_level")
	pturbo_plus_percent := flag.Int("turbo_plus_percent", turbo_plus_percent, "turbo_plus_percent")
	pturbo_plus_percent_level := flag.Int("turbo_plus_percent_level", turbo_plus_percent_level, "turbo_plus_percent_level")
	pturbo_plus_score := flag.Int("turbo_plus_score", turbo_plus_score, "turbo_plus_score")
	pturbo_plus_score_level := flag.Int("turbo_plus_score_level", turbo_plus_score_level, "turbo_plus_score_level")
	cpuprofile := flag.String("cpuprofile", "", "CPU profile 情報保存ファイル名")
	memprofile := flag.String("memprofile", "", "Memory profileサービスを提供するポート")

	flag.Parse()

	auto_mode = *pauto_mode
	calc_gap_mode = *pcalc_gap_mode
	print_mode = *pprint_mode
	print_mode_turbo = *pprint_mode_turbo
	pause_mode = *ppause_mode
	one_time = *pone_time
	seed = *pseed
	turbo_minus_percent = *pturbo_minus_percent
	turbo_minus_percent_level = *pturbo_minus_percent_level
	turbo_minus_score = *pturbo_minus_score
	turbo_minus_score_level = *pturbo_minus_score_level
	turbo_plus_percent = *pturbo_plus_percent
	turbo_plus_percent_level = *pturbo_plus_percent_level
	turbo_plus_score = *pturbo_plus_score
	turbo_plus_score_level = *pturbo_plus_score_level
	if (*cpuprofile != "") {
		f, _ := os.Create(*cpuprofile)
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	if (*memprofile != "") {
		go func() {
			log.Println(http.ListenAndServe("localhost:"+*memprofile, nil))
		}()
	}
	
	fmt.Println("auto_mode=", *pauto_mode)
	fmt.Println("calc_gap_mode=", *pcalc_gap_mode)
	fmt.Println("print_mode=", *pprint_mode)
	fmt.Println("print_mode_turbo=", *pprint_mode_turbo)
	fmt.Println("pause_mode=", *ppause_mode)
	fmt.Println("seed=", *pseed)
	fmt.Println("one_time=", *pone_time)
	fmt.Println("turbo_minus_percent=", *pturbo_minus_percent)
	fmt.Println("turbo_minus_percent_level=", *pturbo_minus_percent_level)
	fmt.Println("turbo_minus_score=", *pturbo_minus_score)
	fmt.Println("turbo_minus_score_level=", *pturbo_minus_score_level)
	fmt.Println("turbo_plus_percent=", *pturbo_plus_percent)
	fmt.Println("turbo_plus_percent_level=", *pturbo_plus_percent_level)
	fmt.Println("turbo_plus_score=", *pturbo_plus_score)
	fmt.Println("turbo_plus_score_level=", *pturbo_plus_score_level)
	fmt.Println("cpuprofile=", *cpuprofile)
	fmt.Println("memprofile=", *memprofile)

	if (seed > 0) {
		rand.Seed(seed)
	} else {
		rand.Seed(time.Now().UnixNano())
	}
	total_start_time = time.Now().UnixNano()
	init_game()
	for {
		gap := moveAuto(auto_mode)
		gen++
		appear()
		disp(gap, print_mode > 0 &&
			(gen%print_mode==0 ||
				(print_mode_turbo==1 && score>turbo_minus_score) ||
				(print_mode_turbo==2 && score>turbo_plus_score)))
		if (isGameOver()) {
			sc := getScore()
			sum_score += sc
			if (sc > max_score) {
				max_score = sc
				max_seed = seed
			}
			if (sc < min_score) {
				min_score = sc
				min_seed = seed
			}
			fmt.Printf("Game Over! (level=%d seed=%d) %s #%d Ave.=%d Max=%d(seed=%d) Min=%d(seed=%d)\ngetGap=%d calcGap=%d %.1f,%.1f %d%%,%d %d,%d %d%%,%d %d,%d %d calc_gap_mode=%d\n",
				auto_mode, seed,
				getTime(), count, sum_score/count,
				max_score, max_seed, min_score, min_seed,
				count_getGap, count_calcGap,
				float64(D_BONUS), float64(GAP_EQUAL),
				turbo_minus_percent, turbo_minus_percent_level,
				turbo_minus_score, turbo_minus_score_level,
				turbo_plus_percent, turbo_plus_percent_level,
				turbo_plus_score, turbo_plus_score_level,
				print_mode_turbo, calc_gap_mode)
			disp(gap, true)
			if (one_time > 0) {
				one_time--;
				if (one_time == 0) {
					break
				}
			}
			if (pause_mode > 0) {
				var key string
				fmt.Scanln(&key)
				if (key == "q") {
					break
				}
			}
			seed++
			rand.Seed(seed)
			init_game()
			count++
		}
	}
	total_last_time = time.Now().UnixNano()
	fmt.Printf("Total time = %.1f (sec)\n", float64(total_last_time-total_start_time)/ticks_per_sec)
}

func getCell(x int, y int) int {
	return (board[x][y])
}

func setCell(x int, y int, n int) int {
	board[x][y] = (n)
	return (n)
}

func clearCell(x int, y int) {
	(setCell(x, y, 0))
}

func copyCell(x1 int, y1 int, x2 int, y2 int) int {
	return (setCell(x2, y2, getCell(x1, y1)))
}

func moveCell(x1 int, y1 int, x2 int, y2 int) {
	copyCell(x1, y1, x2, y2)
	clearCell(x1, y1)
}

func addCell(x1 int, y1 int, x2 int, y2 int) {
	board[x2][y2]++
	clearCell(x1, y1)
	if (sp < 1) {
		addScore(1 << (uint)(getCell(x2, y2)))
	}
}

func isEmpty(x int, y int) bool {
	return (getCell(x, y) == 0)
}

func isNotEmpty(x int, y int) bool {
	return (!isEmpty(x, y))
}

func isGameOver() bool {
	if ret, _, _ := isMovable(); ret {
		return false
	} else {
		return true
	}
}

func getScore() int {
	return (score)
}

func setScore(sc int) int {
	score = (sc)
	return (score)
}

func addScore(sc int) int {
	score += (sc)
	return score
}

func clear() {
	for y := 0; y < YMAX; y++ {
		for x := 0; x < XMAX; x++ {
			clearCell(x, y)
		}
	}
}

func disp(gap float64, debug bool) {
	now := time.Now().UnixNano()
	if (count == 0) {
		fmt.Printf("[%d:%d] %d (%.2f/%.1f sec) %.6f %s seed=%d 2=%.2f%%\r", count, gen, getScore(), float64(now-last_time)/ticks_per_sec, float64(now-start_time)/ticks_per_sec, gap, getTime(), seed, (float64)(count_2)/(float64)(count_2+count_4)*100)
	} else {
		fmt.Printf("[%d:%d] %d (%.2f/%.1f sec) %.6f %s seed=%d 2=%.2f%% Ave.=%d\r", count, gen, getScore(), float64(now-last_time)/ticks_per_sec, float64(now-start_time)/ticks_per_sec, gap, getTime(), seed, (float64)(count_2)/(float64)(count_2+count_4)*100, (sum_score+getScore())/count)
	}
	last_time = now
	if (debug) {
		fmt.Printf("\n")
		for y := 0; y < YMAX; y++ {
			for x := 0; x < XMAX; x++ {
				v := getCell(x, y)
				if (v > 0) {
					fmt.Printf("%5d ", 1<<(uint)(v))
				} else {
					fmt.Printf("%5s ", ".")
				}
			}
			fmt.Printf("\n")
		}
	}
}

func init_game() {
	gen = 1
	setScore(0)
	start_time = time.Now().UnixNano()
	last_time = start_time
	clear()
	appear()
	appear()
	count_2 = 0
	count_4 = 0
	count_calcGap = 0
	count_getGap = 0
	disp(0.0, print_mode == 1)
}

func getTime() string {
	//Mon Jan 2 15:04:05 MST 2006 (MST is GMT-0700)
	return time.Now().Format("2006/01/02 15:04:05")
}

func appear() bool {
	n := 0
	for y := 0; y < YMAX; y++ {
		for x := 0; x < XMAX; x++ {
			if (isEmpty(x, y)) {
				pos_x[n] = x
				pos_y[n] = y
				n++
			}
		}
	}
	if (n> 0) {
		var v int
		i := rand.Intn(65535) % n
		if ((rand.Intn(65535) % RNDMAX) >= 1) {
			v = INIT2
			count_2++
		} else {
			v = INIT4
			count_4++
		}
		x := pos_x[i]
		y := pos_y[i]
		setCell(x, y, v)
		return true
	}
	return false
}

func countEmpty() int {
	var ret int = 0
	for y := 0; y < YMAX; y++ {
		for x := 0; x < XMAX; x++ {
			if (isEmpty(x, y)) {
				ret++
			}
		}
	}
	return ret
}

func move_up() int {
	move := 0
	var yLimit int
	var yNext int
	for x := 0; x < XMAX; x++ {
		yLimit = 0
		for y := 1; y < YMAX; y++ {
			if (isNotEmpty(x, y)) {
				yNext = y - 1
				for yNext >= yLimit {
					if (isNotEmpty(x, yNext)) {
						break
					}
					if (yNext == 0) {
						break
					}
					yNext = yNext - 1
				}
				if (yNext < yLimit) {
					yNext = yLimit
				}
				if (isEmpty(x, yNext)) {
					moveCell(x, y, x, yNext)
					move++
				} else {
					if (getCell(x, yNext) == getCell(x, y)) {
						addCell(x, y, x, yNext)
						move++
						yLimit = yNext + 1
					} else {
						if (yNext+1 != y) {
							moveCell(x, y, x, yNext+1)
							move++
							yLimit = yNext + 1
						}
					}
				}
			}
		}
	}
	return move
}

func move_left() int {
	move := 0
	var xLimit int
	var xNext int
	for y := 0; y < YMAX; y++ {
		xLimit = 0
		for x := 1; x < XMAX; x++ {
			if (isNotEmpty(x, y)) {
				xNext = x - 1
				for xNext >= xLimit {
					if (isNotEmpty(xNext, y)) {
						break
					}
					if (xNext == 0) {
						break
					}
					xNext = xNext - 1
				}
				if (xNext < xLimit) {
					xNext = xLimit
				}
				if (isEmpty(xNext, y)) {
					moveCell(x, y, xNext, y)
					move++
				} else {
					if (getCell(xNext, y) == getCell(x, y)) {
						addCell(x, y, xNext, y)
						move++
						xLimit = xNext + 1
					} else {
						if (xNext+1 != x) {
							moveCell(x, y, xNext+1, y)
							move++
							xLimit = xNext + 1
						}
					}
				}
			}
		}
	}
	return move
}

func move_down() int {
	move := 0
	var yLimit int
	var yNext int
	for x := 0; x < XMAX; x++ {
		yLimit = YMAX_1
		for y := YMAX - 2; y >= 0; y-- {
			if (isNotEmpty(x, y)) {
				yNext = y + 1
				for yNext <= yLimit {
					if (isNotEmpty(x, yNext)) {
						break
					}
					if (yNext == YMAX_1) {
						break
					}
					yNext = yNext + 1
				}
				if (yNext > yLimit) {
					yNext = yLimit
				}
				if (isEmpty(x, yNext)) {
					moveCell(x, y, x, yNext)
					move++
				} else {
					if (getCell(x, yNext) == getCell(x, y)) {
						addCell(x, y, x, yNext)
						move++
						yLimit = yNext - 1
					} else {
						if (yNext-1 != y) {
							moveCell(x, y, x, yNext-1)
							move++
							yLimit = yNext - 1
						}
					}
				}
			}
		}
	}
	return move
}

func move_right() int {
	move := 0
	var xLimit int
	var xNext int
	for y := 0; y < YMAX; y++ {
		xLimit = XMAX_1
		for x := XMAX - 2; x >= 0; x-- {
			if (isNotEmpty(x, y)) {
				xNext = x + 1
				for xNext <= xLimit {
					if (isNotEmpty(xNext, y)) {
						break
					}
					if (xNext == XMAX_1) {
						break
					}
					xNext = xNext + 1
				}
				if (xNext > xLimit) {
					xNext = xLimit
				}
				if (isEmpty(xNext, y)) {
					moveCell(x, y, xNext, y)
					move++
				} else {
					if (getCell(xNext, y) == getCell(x, y)) {
						addCell(x, y, xNext, y)
						move++
						xLimit = xNext - 1
					} else {
						if (xNext-1 != x) {
							moveCell(x, y, xNext-1, y)
							move++
							xLimit = xNext - 1
						}
					}
				}
			}
		}
	}
	return move
}

func moveAuto(autoMode int) float64 {
	empty := countEmpty()
	sc := getScore()
	if (empty >= XMAX*YMAX*turbo_minus_percent/100) {
		autoMode -= turbo_minus_percent_level
	} else if (empty < XMAX*YMAX*turbo_plus_percent/100) {
		autoMode += turbo_plus_percent_level
	}
	if (sc < turbo_minus_score) {
		autoMode -=turbo_minus_score_level
	} else if (sc >= turbo_plus_score) {
		autoMode += turbo_plus_score_level
	}
	return moveBest(autoMode, true)
}

func moveBest(nAutoMode int, move bool) float64 {
	var nGap float64
	var nGapBest float64
	var nDirBest int = 0
	var nDir int = 0
	board_bak := board
	sp++
	nGapBest = GAP_MAX
	if (move_up() > 0) {
		nDir = 1
		nGap = getGap(nAutoMode, nGapBest)
		if (nGap < nGapBest) {
			nGapBest = nGap
			nDirBest = 1
		}
	}
	board = board_bak
	if (move_left() > 0) {
		nDir = 2
		nGap = getGap(nAutoMode, nGapBest)
		if (nGap < nGapBest) {
			nGapBest = nGap
			nDirBest = 2
		}
	}
	board = board_bak
	if (move_down() > 0) {
		nDir = 3
		nGap = getGap(nAutoMode, nGapBest)
		if (nGap < nGapBest) {
			nGapBest = nGap
			nDirBest = 3
		}
	}
	board = board_bak
	if (move_right() > 0) {
		nDir = 4
		nGap = getGap(nAutoMode, nGapBest)
		if (nGap < nGapBest) {
			nGapBest = nGap
			nDirBest = 4
		}
	}
	board = board_bak
	sp--
	if (move) {
		if (nDirBest == 0) {
			fmt.Printf("\n***** Give UP *****\n")
			nDirBest = nDir
		}
		switch nDirBest {
		case 1:
			move_up()
			break
		case 2:
			move_left()
			break
		case 3:
			move_down()
			break
		case 4:
			move_right()
			break
		}
	}
	return nGapBest
}

func getGap(nAutoMode int, nGapBest float64) float64 {
	count_getGap++
	var ret float64 = 0.0
	movable, nEmpty, nBonus := isMovable()
	if (! movable) {
		ret = GAP_MAX
	} else if (nAutoMode <= 1) {
		ret = getGap1(nGapBest, nEmpty, nBonus)
	} else {
		alpha := nGapBest * float64(nEmpty) //累積がこれを超えれば、平均してもnGapBestを超えるので即枝刈りする
		for x := 0; x < XMAX; x++ {
			for y := 0; y < YMAX; y++ {
				if (isEmpty(x, y)) {
					setCell(x, y, INIT2)
					ret += moveBest(nAutoMode-1, false) * (RNDMAX - 1) / RNDMAX
					if (ret >= alpha) {
						return GAP_MAX	//枝刈り
					}
					setCell(x, y, INIT4)
					ret += moveBest(nAutoMode-1, false) / RNDMAX
					if (ret >= alpha) {
						return GAP_MAX	//枝刈り
					}
					clearCell(x, y)
				}
			}
		}
		ret /= float64(nEmpty) //平均値を返す
	}
	return ret
}

func getGap1(nGapBest float64, nEmpty int, nBonus float64) float64 {
	var ret float64 = 0.0
	var ret_appear float64 = 0.0
	alpha := nGapBest * nBonus
	edgea := false
	edgeb := false;
	for x := 0; x < XMAX; x++ {
		for y := 0; y < YMAX; y++ {
			v := getCell(x, y)
			edgea = (x == 0 || y == 0) || (x == XMAX - 1 || y == YMAX_1)
			if (v > 0) {
				if (x < XMAX_1) {
					x1 := getCell(x+1, y)
					edgeb = (y == 0) || (x+1 == XMAX - 1 || y == YMAX_1)
					if (x1 > 0) {
						ret += calcGap(v, x1, edgea, edgeb)
					} else {
						ret_appear += calcGap(v, INIT2, edgea, edgeb) * (RNDMAX - 1) / RNDMAX
						ret_appear += calcGap(v, INIT4, edgea, edgeb) / RNDMAX
					}
				}
				if (y < YMAX_1) {
					y1 := getCell(x, y+1)
					edgeb = (x == 0) || (x == XMAX - 1 || y+1 == YMAX_1)
					if (y1 > 0) {
						ret += calcGap(v, y1, edgea, edgeb)
					} else {
						ret_appear += calcGap(v, INIT2, edgea, edgeb) * (RNDMAX - 1) / RNDMAX
						ret_appear += calcGap(v, INIT4, edgea, edgeb) / RNDMAX
					}
				}
			} else {
				if (x < XMAX_1) {
					x1 := getCell(x+1, y)
					edgeb = (y == 0) || (x+1 == XMAX - 1 || y == YMAX_1)
					if (x1 > 0) {
						ret_appear += calcGap(INIT2, x1, edgea, edgeb) * (RNDMAX - 1) / RNDMAX
						ret_appear += calcGap(INIT4, x1, edgea, edgeb) / RNDMAX
					}
				}
				if (y < YMAX_1) {
					y1 := getCell(x, y+1)
					edgeb = (x == 0) || (x == XMAX - 1 || y+1 == YMAX_1)
					if (y1 > 0) {
						ret_appear += calcGap(INIT2, y1, edgea, edgeb) * (RNDMAX - 1) / RNDMAX
						ret_appear += calcGap(INIT4, y1, edgea, edgeb) / RNDMAX
					}
				}
			}
			if (ret + ret_appear/float64(nEmpty) > alpha) {
				return GAP_MAX
			}
		}
	}
	ret += ret_appear / float64(nEmpty)
	ret /= nBonus
	return ret
}

func calcGap(a int, b int, edgea bool, edgeb bool) float64 {
	count_calcGap++
	var ret float64 = 0
	if (a > b) {
		ret = float64(a - b)
		if (calc_gap_mode > 0 && ! edgea && edgeb) {
			switch (calc_gap_mode) {
			case 1:
				ret += 1
				break
			case 2:
				ret *= 2
				break
			case 3:
				ret += float64(a)
				break
			case 4:
				ret += float64(a)/10
				break
			case 5:
				ret += float64(a+b)
				break
			}
		}
	} else if (a < b) {
		ret = float64(b - a)
		if (calc_gap_mode > 0 && edgea && ! edgeb) {
			switch (calc_gap_mode) {
			case 1:
				ret += 1
			case 2:
				ret *= 2
			case 3:
				ret += float64(b)
			case 4:
				ret += float64(b)/10
			case 5:
				ret += float64(a+b)
				break
			}
		}
	} else {
		ret = GAP_EQUAL
	}
	return ret
}

func isMovable() (bool, int, float64) {
	ret := false //動けるか？
	nEmpty := 0 //空きの数
	var nBonus float64 = 1.0 //ボーナス（隅が最大値ならD_BONUS）
	var max_x, max_y int
	max := 0
	for y := 0; y < YMAX; y++ {
		for x := 0; x < XMAX; x++ {
			val := getCell(x, y)
			if (val == 0) {
				ret = true
				nEmpty++
			} else {
				if (val > max) {
					max = val
					max_x = x
					max_y = y
				}
				if (! ret) {
					if (x < XMAX_1) {
						x1 := getCell(x+1, y)
						if (val == x1 || x1 == 0) {
							ret = true
						}
					}
					if (y < YMAX_1) {
						y1 := getCell(x, y+1)
						if (val == y1 || y1 == 0) {
							ret = true
						}
					}
				}
			}
		}
	}
	if ((max_x == 0 || max_x == XMAX_1) &&
		(max_y == 0 || max_y == YMAX_1)) {
		if (D_BONUS_USE_MAX) {
			nBonus = float64(max)
		} else {
			nBonus = D_BONUS
		}
	}
	return ret, nEmpty, nBonus
}
