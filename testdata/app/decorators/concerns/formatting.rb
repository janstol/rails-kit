module Formatting
  def formatted_amount(cents)
    "$#{'%.2f' % (cents / 100.0)}"
  end

  def formatted_date(date)
    date.strftime("%B %d, %Y")
  end
end
