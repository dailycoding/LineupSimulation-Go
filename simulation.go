package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/go-gota/gota/dataframe"
	"github.com/go-gota/gota/series"
	"github.com/xuri/excelize/v2"
)

func checkDuplicatesInArray(values []int) bool {
	visited := make(map[int]bool, 0)
	for i := 0; i < len(values); i++ {
		if visited[values[i]] == true {
			return true
		} else {
			visited[values[i]] = true
		}
	}
	return false
}

func findIndexInSeries(value string, arr series.Series) int {
	for i := 0; i < arr.Len(); i++ {
		if arr.Elem(i).String() == value {
			return i
		}
	}
	return -1
}

func randomChoice(a []int, p []float64) int {
	t := p
	for i := 1; i < len(t); i++ {
		t[i] += t[i-1]
	}
	r := rand.Float64() * p[len(p)-1]
	for i := 0; i < len(p); i++ {
		if r < p[i] {
			return a[i]
		}
	}
	return 0
}

func rankify(values []float64) []float64 {
	// Rank Vector
	var R = []float64{}

	// Sweep through all elements in A for each
	// element count the number of less than and
	// equal elements separately in r and s.
	for i := 0; i < len(values); i++ {
		r := 1
		s := 1

		for j := 0; j < len(values); j++ {
			if j != i && values[j] < values[i] {
				r += 1
			}
			if j != i && values[j] == values[i] {
				s += 1
			}
		}

		// Use formula to obtain rank
		R = append(R, float64(r)+float64(s-1)/2.0)
	}
	return R
}

func toJsonString(arr []string) string {
	b := new(strings.Builder)
	json.NewEncoder(b).Encode(arr)
	return strings.Trim(b.String(), "\n")
}

//  Computing new probability of a player
func computePlayerProb(playerName string, resultdf dataframe.DataFrame) float64 {
	var totalTeams = resultdf.Nrow()
	var playerPresenceCount = 0
	playerName = "\"" + playerName + "\""
	for _, team := range resultdf.Col("LineupMembers").Records() {
		// team is string of json array
		index := strings.Index(team, playerName)
		if index < 0 {
			continue
		} else {
			playerPresenceCount += 1
		}
	}

	var probability = float64(playerPresenceCount) / float64(totalTeams)
	return probability
}

// Computing Problabilites of all Players
func computeProbabilities(dfProb dataframe.DataFrame, resultdf dataframe.DataFrame) dataframe.DataFrame {
	// tempDfProb = dfProb
	var probabilities = []float64{}
	for _, name := range dfProb.Col("Name").Records() {
		probabilities = append(probabilities, computePlayerProb(name, resultdf))
	}
	dfProb = dfProb.Mutate(
		series.New(probabilities, series.Float, "probabilities"),
	)

	return dfProb
}

// Computing Rank value by summing the values in simulations
func computeRankValue(team string, simulation dataframe.DataFrame) float64 {
	netRankValue := 0.0
	var teamMembers []string
	json.Unmarshal([]byte(team), &teamMembers)
	for _, teamMember := range teamMembers {
		i := findIndexInSeries(teamMember, simulation.Col("player_name"))
		value := simulation.Elem(i, 1).Float()
		netRankValue += value
	}
	return netRankValue
}

// Running all simulations
func runSimulations(resultdf dataframe.DataFrame, simdf dataframe.DataFrame, numberOfSimulations int) dataframe.DataFrame {
	for i := 0; i < numberOfSimulations; i++ {
		colName := strconv.Itoa(i)

		//os.system("cls")
		fmt.Println("Running Simulations ", i, " of ", numberOfSimulations)
		simulation := simdf.Select([]string{"player_name", colName})

		var rankValues = []float64{}
		for j := 0; j < resultdf.Nrow(); j++ {
			rankValue := computeRankValue(resultdf.Col("LineupMembers").Elem(j).String(), simulation)
			rankValues = append(rankValues, rankValue)
		}

		// Converting values into ranks
		ranked := rankify(rankValues)
		ranked_max := float64(len(rankValues) + 1)
		for k, v := range ranked {
			ranked[k] = ranked_max - v
		}

		resultdf = resultdf.Mutate(
			series.New(ranked, series.Float, colName),
		)
	}

	fmt.Println("Simulations Completed")
	return resultdf
}

func new_lineups_dataframe() dataframe.DataFrame {
	return dataframe.New(
		series.New([]string{}, series.String, "Name"),
		series.New([]float64{}, series.Float, "Salary"),
	)
}

func generateSmallLineups(numberOfTeams int, playersPerTeam int, df dataframe.DataFrame) dataframe.DataFrame {
	df = df.Copy()
	df = df.Arrange(
		dataframe.RevSort("probabilities"),
	)
	fmt.Println(df)
	var names = df.Col("Name")
	var probs = df.Col("probabilities")
	var lineups_list = [][]string{}

	for len(lineups_list) < numberOfTeams {
		var new_lineups = new_lineups_dataframe()
		// var lineupSalary = 0
		var generator = []int{0, 1}

		for new_lineups.Nrow() < playersPerTeam {
			for i := 0; i < df.Nrow(); i++ {
				var name = names.Elem(i).String()
				var probability = probs.Elem(i).Float()
				var selection = randomChoice(generator, []float64{(1 - probability), probability})
				if new_lineups.Nrow() == playersPerTeam {
					break
				}

				if selection == 0 {
					continue
				} else {
					lineup_names := new_lineups.Col("Name").Records()
					lineup_names = append(lineup_names, name)
					salaries := df.Filter(
						dataframe.F{Colname: "Name", Comparator: series.In, Comparando: lineup_names},
					)
					total_salary := 0
					salary_values, _ := salaries.Col("Salary").Int()
					for _, salary := range salary_values {
						total_salary += salary
					}
					new_lineups = new_lineups.Concat(dataframe.New(
						series.New([]string{name}, series.String, "Name"),
						series.New([]float64{float64(total_salary)}, series.Float, "Salary"),
					))
				}

				last_total_salary := new_lineups.Col("Salary").Elem(new_lineups.Nrow() - 1).Float()
				if last_total_salary > 50000 {
					new_lineups = new_lineups_dataframe()
					continue
				}

				if new_lineups.Nrow() == playersPerTeam && last_total_salary < 48000 {
					new_lineups = new_lineups_dataframe()
					continue
				}
			}
		}

		// lineup created
		lineup_names := new_lineups.Col("Name").Records()

		// #sort players in a lineup on basis of salary
		for i := 0; i < len(lineup_names); i++ {
			name_index_i := findIndexInSeries(lineup_names[i], names)
			for j := i + 1; j < len(lineup_names); j++ {
				name_index_j := findIndexInSeries(lineup_names[j], names)
				if df.Col("Salary").Elem(name_index_i).Float() > df.Col("Salary").Elem(name_index_j).Float() {
					temp := lineup_names[i]
					lineup_names[i] = lineup_names[j]
					lineup_names[j] = temp
				}
			}
		}

		lineups_list = append(lineups_list, lineup_names)
	}

	// Computing Team Salaries
	lineupNames := []string{}
	lineupSalaries := []int{}
	for i := 0; i < len(lineups_list); i++ {
		lineupSalary := 0
		for _, member_name := range lineups_list[i] {
			j := findIndexInSeries(member_name, names)
			if j >= 0 {
				lineupSalary += int(df.Col("Salary").Elem(j).Float())
			}
		}

		lineupNames = append(lineupNames, toJsonString(lineups_list[i]))
		lineupSalaries = append(lineupSalaries, lineupSalary)
	}

	resultdf := dataframe.New(
		series.New(lineupNames, series.String, "LineupMembers"),
		series.New(lineupSalaries, series.Int, "LineupSalary"),
	)

	// Sort teams on the basis of salaries
	resultdf = resultdf.Arrange(
		dataframe.RevSort("LineupSalary"),
	)

	fmt.Println("resultdf", resultdf)

	return resultdf
}

func toInt2dArray(values dataframe.DataFrame) [][]int {
	var output = [][]int{}
	for r := 0; r < values.Nrow(); r++ {
		var row = []int{}
		for c := 0; c < values.Ncol(); c++ {
			v, e := values.Elem(r, c).Int()
			if e != nil {
				row = append(row, 0)
			} else {
				row = append(row, v)
			}
		}
		output = append(output, row)
	}
	return output
}

func numpy_bincount(values []int) []int {
	var m = make(map[int]int)
	for _, v := range values {
		count, exists := m[v]
		if exists {
			m[v] = count + 1
		} else {
			m[v] = 1
		}
	}

	var bincount = []int{}
	for _, v := range values {
		bincount = append(bincount, m[v])
	}

	return bincount
}

func saveAsCSV(filename string, df dataframe.DataFrame) (bool, error) {
	f, err := os.Create(filename)
	if err != nil {
		return false, err
	}
	df.WriteCSV(f)
	f.Close()
	return true, nil
}

func main() {
	const playersPerTeam = 6
	const numberOfSimulations = 10000

	var numberOfTeams int = 0
	fmt.Print("Please Enter the number of teams you want to create: ")
	fmt.Scanf("%d", &numberOfTeams)
	fmt.Println("Number of Teams:", numberOfTeams)
	if numberOfTeams <= 0 {
		return
	}

	// Reading Data from Excel file
	xls, err := excelize.OpenFile("lineup_simulations.xlsx")
	if err != nil {
		fmt.Println("Error reading file, program terminating...")
		return
	}

	rows, err := xls.GetRows("Salaries_probabilities")
	if err != nil {
		fmt.Println(err)
		return
	}
	var df = dataframe.LoadRecords(rows)
	// var df1 = df.Select([]string{"Name", "Salary"})
	var df2 = df.Select([]string{"Name", "Salary", "probabilities"})

	var resultdf = generateSmallLineups(numberOfTeams, playersPerTeam, df2)
	fmt.Println("Resultant Lineups", resultdf)

	// Computing player new probabilities
	var dfProb = df.Select([]string{"Name", "probabilities"})
	dfProb = computeProbabilities(dfProb, resultdf)

	// Writing new probabilities results to files
	_, err = saveAsCSV("dfProb.csv", dfProb)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println(dfProb)

	// -------------Running the simulation---------------------------
	// Reading Data from Excel file
	rows, err = xls.GetRows("simulations")
	if err != nil {
		fmt.Println("Error reading file, program terminating...")
		return
	}
	var simdf = dataframe.LoadRecords(rows)

	// If player names do not match in player and simulation file then exit
	var diff = simdf.Col("player_name").Compare(series.Eq, df.Col("Name"))
	for _, e := range diff.Records() {
		if e != "true" {
			fmt.Println("Names in Salaries and simulations are not same, Program terminating...")
			return
		}
	}

	// Run Simulations
	resultdf = runSimulations(resultdf, simdf, numberOfSimulations)
	fmt.Println(resultdf)

	// Computing expected values
	var resultdfCopy = resultdf.Drop([]string{"LineupMembers", "LineupSalary"})
	// numpy_array contains all the rankings from the simulations
	var numpy_array = toInt2dArray(resultdfCopy)
	fmt.Println(numpy_array)

	// Reading Payout Data from Excel file
	rows, err = xls.GetRows("payouts")
	if err != nil {
		fmt.Println("Error reading file, program terminating...")
		return
	}
	var payoutsdf = dataframe.LoadRecords(rows)
	payoutsdf = payoutsdf.Drop([]string{"lineup_rank", "associated_payout"})
	var net_pay = toInt2dArray(payoutsdf)

	// Creating bins of rankings of teams
	var count_arr = [][]int{}
	for _, arr := range numpy_array {
		count_arr = append(count_arr, numpy_bincount(arr))
	}

	var count_arr_float = [][]float64{}

	// converting counts to percentage
	for i := 0; i < numberOfTeams; i++ {
		temp := []float64{}
		for j := 0; j <= numberOfTeams; j++ {
			temp = append(temp, float64(count_arr[i][j])/float64(numberOfSimulations))
		}
		count_arr_float = append(count_arr_float, temp)
	}
	fmt.Println("percent", count_arr_float)

	// Computing Expected value by taking sum product of Percentages and net payout
	var sumProduct = 0.0
	var expectedValues = []float64{}
	for i := 0; i < resultdf.Col("LineupMembers").Len(); i++ {
		for j := 0; j <= numberOfTeams; j++ {
			netPay := 0.0
			if j == 0 {
				netPay = 0.0
			} else if j <= 10 {
				netPay = float64(net_pay[j-1][0])
			} else {
				netPay = float64(net_pay[10][0])
			}
			sumProduct += count_arr_float[i][j] * netPay
		}
		fmt.Println("Team ", resultdf.Col("LineupMembers").Elem(i), " ", sumProduct)
		expectedValues = append(expectedValues, sumProduct)
	}
	resultdf = resultdf.Mutate(
		series.New(expectedValues, series.Float, "ExpectedValue"),
	)

	// Only keep the first and last columns of the dataframe
	resultdf = resultdf.Select([]string{"LineupMembers", "LineupSalary", "ExpectedValue"})

	// Sort the dataframe on the basis of Expected Value
	resultdf = resultdf.Arrange(
		dataframe.RevSort("ExpectedValue"),
	)

	saveAsCSV("Expectedresults3.csv", resultdf)
	fmt.Println(resultdf)

	return
}
