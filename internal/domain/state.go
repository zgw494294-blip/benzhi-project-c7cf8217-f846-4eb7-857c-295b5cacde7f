package domain

var stateTransitions = map[VolumeState]map[VolumeState]bool{
	StateDraft:           {StateTranscribing: true},
	StateTranscribing:    {StateChecking: true},
	StateChecking:        {StateNeedsCorrection: true, StateReadyForReview: true},
	StateNeedsCorrection: {StateChecking: true},
	StateReadyForReview:  {StateTranscribing: true, StateChecking: true, StateFrozen: true},
	StateFrozen:          {StateAccessioned: true},
	StateAccessioned:     {},
}

func CanTransition(from, to VolumeState) bool {
	return stateTransitions[from][to]
}

func (v *DigitizationVolume) Transition(to VolumeState) error {
	if v.State == to {
		return nil
	}
	if !CanTransition(v.State, to) {
		return NewRuleError(CodeForbidden, "卷状态不能从 %s 变更为 %s", v.State, to)
	}
	v.State = to
	return nil
}

func (v *DigitizationVolume) EnsureEditable() error {
	if v.State == StateFrozen || v.State == StateAccessioned {
		return NewRuleError(CodeForbidden, "卷已冻结，不能继续普通编辑")
	}
	if v.State == StateChecking {
		return NewRuleError(CodeConflict, "完整性检查进行中，不能编辑")
	}
	return nil
}
