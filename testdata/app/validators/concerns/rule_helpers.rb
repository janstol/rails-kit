module RuleHelpers
  def blank_or_matches?(value, format)
    value.blank? || value.match?(format)
  end
end
