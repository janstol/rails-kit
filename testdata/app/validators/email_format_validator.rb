class EmailFormatValidator < ActiveModel::EachValidator
  FORMAT = /\A[^@\s]+@[^@\s]+\z/

  def validate_each(record, attribute, value)
    return if value.blank?
    return if value.match?(FORMAT)

    record.errors.add(attribute, options[:message] || "is not a valid email")
  end

  def self.default_message
    "is not a valid email"
  end

  private

  def normalized(value)
    value.to_s.strip.downcase
  end
end
