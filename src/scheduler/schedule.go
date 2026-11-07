package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type taskSchedule struct {
	every                             time.Duration
	minute, hour, day, month, weekday map[int]bool
	dayAny, weekdayAny                bool
}

func parseTaskSchedule(expression string) (*taskSchedule, error) {
	expression = strings.TrimSpace(expression)
	if strings.HasPrefix(expression, "@every ") {
		duration, err := time.ParseDuration(strings.TrimSpace(strings.TrimPrefix(expression, "@every ")))
		if err != nil || duration <= 0 {
			return nil, fmt.Errorf("invalid interval %q", expression)
		}
		return &taskSchedule{every: duration}, nil
	}
	aliases := map[string]string{
		"@yearly": "0 0 1 1 *", "@annually": "0 0 1 1 *", "@monthly": "0 0 1 * *",
		"@weekly": "0 0 * * 0", "@daily": "0 0 * * *", "@midnight": "0 0 * * *", "@hourly": "0 * * * *",
	}
	if value, ok := aliases[strings.ToLower(expression)]; ok {
		expression = value
	}
	fields := strings.Fields(expression)
	if len(fields) != 5 {
		return nil, fmt.Errorf("expected five cron fields or a supported descriptor")
	}
	minute, _, err := parseScheduleField(fields[0], 0, 59, nil, false)
	if err != nil {
		return nil, fmt.Errorf("minute: %w", err)
	}
	hour, _, err := parseScheduleField(fields[1], 0, 23, nil, false)
	if err != nil {
		return nil, fmt.Errorf("hour: %w", err)
	}
	day, dayAny, err := parseScheduleField(fields[2], 1, 31, nil, false)
	if err != nil {
		return nil, fmt.Errorf("day: %w", err)
	}
	monthNames := scheduleNames("jan feb mar apr may jun jul aug sep oct nov dec")
	month, _, err := parseScheduleField(fields[3], 1, 12, monthNames, false)
	if err != nil {
		return nil, fmt.Errorf("month: %w", err)
	}
	weekdayNames := map[string]int{"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6}
	weekday, weekdayAny, err := parseScheduleField(fields[4], 0, 7, weekdayNames, true)
	if err != nil {
		return nil, fmt.Errorf("weekday: %w", err)
	}
	return &taskSchedule{minute: minute, hour: hour, day: day, month: month, weekday: weekday, dayAny: dayAny, weekdayAny: weekdayAny}, nil
}

// ValidateTaskSchedule reports whether expression uses a schedule supported
// by the in-process scheduler.
func ValidateTaskSchedule(expression string) error {
	_, err := parseTaskSchedule(expression)
	return err
}

func scheduleNames(names string) map[string]int {
	result := make(map[string]int)
	for i, name := range strings.Fields(names) {
		result[name] = i + 1
	}
	return result
}

func parseScheduleField(raw string, minValue, maxValue int, names map[string]int, sundaySeven bool) (map[int]bool, bool, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	any := raw == "*" || raw == "?"
	wildcard := any || strings.HasPrefix(raw, "*/")
	values := make(map[int]bool)
	if any {
		for value := minValue; value <= maxValue; value++ {
			addScheduleValue(values, value, sundaySeven)
		}
		return values, true, nil
	}
	for _, part := range strings.Split(raw, ",") {
		step := 1
		base := part
		if pieces := strings.Split(part, "/"); len(pieces) == 2 {
			var err error
			step, err = strconv.Atoi(pieces[1])
			if err != nil || step <= 0 {
				return nil, false, fmt.Errorf("invalid step %q", part)
			}
			base = pieces[0]
		} else if len(pieces) > 2 {
			return nil, false, fmt.Errorf("invalid field %q", part)
		}
		start, end := minValue, maxValue
		if base != "" && base != "*" && base != "?" {
			if bounds := strings.Split(base, "-"); len(bounds) == 2 {
				var err error
				start, err = scheduleValue(bounds[0], names)
				if err != nil {
					return nil, false, err
				}
				end, err = scheduleValue(bounds[1], names)
				if err != nil {
					return nil, false, err
				}
			} else if len(bounds) == 1 {
				var err error
				start, err = scheduleValue(base, names)
				if err != nil {
					return nil, false, err
				}
				end = start
				if step > 1 {
					end = maxValue
				}
			} else {
				return nil, false, fmt.Errorf("invalid range %q", part)
			}
		}
		if start < minValue || start > maxValue || end < minValue || end > maxValue {
			return nil, false, fmt.Errorf("value out of range in %q", part)
		}
		if start <= end {
			for value := start; value <= end; value += step {
				addScheduleValue(values, value, sundaySeven)
			}
		} else if sundaySeven {
			for value := start; value <= maxValue; value += step {
				addScheduleValue(values, value, true)
			}
			for value := minValue; value <= end; value += step {
				addScheduleValue(values, value, true)
			}
		} else {
			return nil, false, fmt.Errorf("descending range %q", part)
		}
	}
	if len(values) == 0 {
		return nil, false, fmt.Errorf("field selects no values")
	}
	return values, wildcard, nil
}

func scheduleValue(raw string, names map[string]int) (int, error) {
	if value, ok := names[raw]; ok {
		return value, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid value %q", raw)
	}
	return value, nil
}

func addScheduleValue(values map[int]bool, value int, sundaySeven bool) {
	if sundaySeven && value == 7 {
		value = 0
	}
	values[value] = true
}

func (s *taskSchedule) next(after time.Time, location *time.Location) time.Time {
	if location == nil {
		location = time.Local
	}
	if s.every > 0 {
		return after.Add(s.every).In(location)
	}
	candidate := after.In(location).Truncate(time.Minute).Add(time.Minute)
	limit := candidate.AddDate(6, 0, 0)
	for candidate.Before(limit) {
		if s.month[int(candidate.Month())] && s.hour[candidate.Hour()] && s.minute[candidate.Minute()] && s.matchesDay(candidate) {
			return candidate
		}
		candidate = candidate.Add(time.Minute)
	}
	return time.Time{}
}

func (s *taskSchedule) matchesDay(candidate time.Time) bool {
	dayMatch := s.day[candidate.Day()]
	weekdayMatch := s.weekday[int(candidate.Weekday())]
	if s.dayAny || s.weekdayAny {
		return dayMatch && weekdayMatch
	}
	return dayMatch || weekdayMatch
}
